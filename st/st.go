package st

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/resource"
)

var Model = resource.NewModel("appliedmotion", "motor", "st")

type ST struct {
	resource.Named
	mu          sync.RWMutex
	logger      golog.Logger
	cancelCtx   context.Context
	cancelFunc  func()
	comm        CommPort
	props       motor.Properties
	minRpm      float64
	maxRpm      float64
	stepsPerRev int64
}

// Investigate:
// BS - buffer status
// CE - Communication Error

func init() {
	resource.RegisterComponent(
		board.API,
		Model,
		resource.Registration[motor.Motor, *Config]{Constructor: NewMotor})
}

func NewMotor(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (motor.Motor, error) {
	logger.Info("Starting Applied Motion Products ST-IP Driver v0.1")
	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	b := ST{
		Named:      conf.ResourceName().AsNamed(),
		logger:     logger,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		mu:         sync.RWMutex{},
	}

	if err := b.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return &b, nil
}

func (b *ST) Reconfigure(ctx context.Context, _ resource.Dependencies, conf resource.Config) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	// In case the module has changed name
	b.Named = conf.ResourceName().AsNamed()

	// This never really changes, but we'll set it anyway for completeness
	b.props = motor.Properties{
		PositionReporting: true,
	}

	// Update the min/max RPM
	b.minRpm = newConf.MinRpm
	b.maxRpm = newConf.MaxRpm

	// Update the steps per rev
	b.stepsPerRev = newConf.StepsPerRev

	// Check if the comm object exists at all, if not, create it, this is because we're overloading
	// this reconfigure method and using it during construction as well.
	if b.comm == nil {
		if comm, err := getComm(b.cancelCtx, newConf, b.logger); err != nil {
			return err
		} else {
			b.comm = comm
		}
	}

	// Check if the current config matches the new config, if not, replace the comm object
	// This should be a no-op on the first run through
	if b.comm.GetUri() != newConf.URI {
		b.comm.Close()
		if newComm, err := getComm(b.cancelCtx, newConf, b.logger); err != nil {
			return err
		} else {
			b.comm = newComm
		}
	}

	return nil
}

func getComm(ctx context.Context, conf *Config, logger golog.Logger) (CommPort, error) {
	if strings.ToLower(conf.Protocol) == "ip" {
		timeout := time.Duration(conf.ConnectTimeout * int64(time.Second))
		return newIpComm(ctx, conf.URI, timeout, logger)
	}
	if strings.ToLower(conf.Protocol) == "rs485" {
		return nil, fmt.Errorf("unsupported comm type %s", conf.Protocol)
	}
	if strings.ToLower(conf.Protocol) == "rs232" {
		return nil, fmt.Errorf("unsupported comm type %s", conf.Protocol)
	}
	return nil, fmt.Errorf("unknown comm type %s", conf.Protocol)
}

func (s *ST) getStatus(ctx context.Context) ([]byte, error) {
	if resp, err := s.comm.Send(ctx, "SC"); err != nil {
		return nil, err
	} else {

		// Response format: "\x00\aSC=0009{63\r"
		// we need to strip off the command and any leading or trailing stuff
		startIndex := strings.Index(resp, "=")
		if startIndex == -1 {
			return nil, fmt.Errorf("unable to find response data in %v", resp)
		}
		endIndex := strings.Index(resp, "{")
		if endIndex == -1 {
			endIndex = startIndex + 5
		}

		resp = resp[startIndex+1 : endIndex]
		if val, err := hex.DecodeString(resp); err != nil {
			return nil, err
		} else {
			if len(val) > 4 {
				return nil, errors.New("unexpected status length")
			}
			return val, nil
		}
	}
}

func isMoving(status []byte) bool {
	return (status[1]>>4)&1 == 1
}

func inPosition(status []byte) bool {
	return (status[1]>>3)&1 == 1
}

func (s *ST) getBufferStatus(ctx context.Context) (int, error) {
	if resp, err := s.comm.Send(ctx, "BS"); err != nil {
		return -1, err
	} else {
		// The response should look something like BS=<num>\r
		startIndex := strings.Index(resp, "=")
		if startIndex == -1 {
			return -1, fmt.Errorf("unable to find response data in %v", resp)
		}
		endIndex := strings.Index(resp, "{")
		if endIndex == -1 {
			endIndex = startIndex + 5
		}

		resp = resp[startIndex+1 : endIndex]
		return strconv.Atoi(resp)
	}
}

func (s *ST) waitForMoveCommandToComplete(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return errors.New("context cancelled")
		default:
			if bufferIsEmpty, err := s.isBufferEmpty(ctx); err != nil {
				return err
			} else {
				if isMoving, err := s.IsMoving(ctx); err != nil {
					return err
				} else {
					if bufferIsEmpty && !isMoving {
						return nil
					}
				}
			}
		}
	}
}

func (s *ST) isBufferEmpty(ctx context.Context) (bool, error) {
	b, e := s.getBufferStatus(ctx)
	return b == 63, e
}

func (s *ST) Close(ctx context.Context) error {
	return s.comm.Close()
}

func (s *ST) GoFor(ctx context.Context, rpm float64, positionRevolutions float64, extra map[string]interface{}) error {
	// FL

	// need to convert from revs to steps
	positionSteps := int64(positionRevolutions * float64(s.stepsPerRev))

	// First set the desired position
	if _, err := s.comm.Send(ctx, fmt.Sprintf("DI%v", positionSteps)); err != nil {
		return err
	}
	revSec := rpm / 60
	if _, err := s.comm.Send(ctx, fmt.Sprintf("VE%v", revSec)); err != nil {
		return err
	}

	// Then actually execute the move
	if _, err := s.comm.Send(ctx, "FL"); err != nil {
		// If the board errors here, is it wise to potentially just leave it running?
		return err
	}
	return s.waitForMoveCommandToComplete(ctx)
}

func (s *ST) GoTo(ctx context.Context, rpm float64, positionRevolutions float64, extra map[string]interface{}) error {
	// FP?
	// For Ethernet drives, do not use FP with a position parameter. Instead, use DI to set the target position.
	// I guess this means run:
	// 	DI8000
	// 	FP

	// need to convert from revs to steps
	positionSteps := int64(positionRevolutions * float64(s.stepsPerRev))

	// Set the distance first
	if _, err := s.comm.Send(ctx, fmt.Sprintf("DI%v", positionSteps)); err != nil {
		return err
	}

	// Now set the velocity
	revSec := rpm / 60
	if _, err := s.comm.Send(ctx, fmt.Sprintf("VE%v", revSec)); err != nil {
		return err
	}

	// Now execute the move command
	if _, err := s.comm.Send(ctx, "FP"); err != nil {
		return err
	}

	// Now wait for the command to finish
	return s.waitForMoveCommandToComplete(ctx)
}

func (s *ST) IsMoving(ctx context.Context) (bool, error) {
	status, err := s.getStatus(ctx)
	return isMoving(status), err
}

// IsPowered implements motor.Motor.
func (s *ST) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
	status, err := s.getStatus(ctx)
	return isMoving(status), 0, err
}

// Position implements motor.Motor.
func (s *ST) Position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	// EP?
	return 0, nil
}

// Properties implements motor.Motor.
func (s *ST) Properties(ctx context.Context, extra map[string]interface{}) (motor.Properties, error) {
	return s.props, nil
}

// ResetZeroPosition implements motor.Motor.
func (s *ST) ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	// EP0?
	// SP0?
	// The docs seem to indicate that for proper reset to 0, you must send both EP0 and SP0
	return nil
}

// SetPower implements motor.Motor.
func (s *ST) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	// VE? This is in rev/sec
	desiredRpm := s.maxRpm * powerPct
	s.logger.Warn("SetPower called on motor that uses rotational velocity. Scaling %v based on max Rpm %v. Resulting power: %v", powerPct, s.maxRpm, desiredRpm)
	_, err := s.comm.Send(ctx, fmt.Sprintf("VE%v", desiredRpm))
	if err != nil {
		return err
	}
	return nil
}

// Stop implements motor.Motor.
func (s *ST) Stop(ctx context.Context, extras map[string]interface{}) error {
	// SK - Stop & Kill? Stops and erases queue
	// SM - Stop Move? Stops and leaves queue intact?
	// ST - Halts the current buffered command being executed, but does not affect other buffered commands in the command buffer
	_, err := s.comm.Send(ctx, "SC")
	if err != nil {
		return err
	}
	return nil
}
