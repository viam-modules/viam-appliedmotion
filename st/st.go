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

var Model = resource.NewModel("viam-labs", "appliedmotion", "st")

type ST struct {
	resource.Named
	mu           sync.RWMutex
	logger       golog.Logger
	cancelCtx    context.Context
	cancelFunc   func()
	comm         CommPort
	minRpm       float64
	maxRpm       float64
	acceleration float64
	deceleration float64
	stepsPerRev  int64
}

var ErrStatusMessageIncorrectLength = errors.New("status message incorrect length")

// Investigate:
// CE - Communication Error

func init() {
	resource.RegisterComponent(
		board.API,
		Model,
		resource.Registration[motor.Motor, *Config]{Constructor: NewMotor})
}

func NewMotor(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (motor.Motor, error) {
	logger.Info("Starting Applied Motion Products ST Motor Driver v0.1")
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
	b.logger.Debug("Reconfiguring Applied Motion Products ST Motor Driver")

	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	// In case the module has changed name
	b.Named = conf.ResourceName().AsNamed()

	// Update the min/max RPM
	b.minRpm = newConf.MinRpm
	b.maxRpm = newConf.MaxRpm

	// Update the steps per rev
	b.stepsPerRev = newConf.StepsPerRev

	b.acceleration = newConf.Acceleration
	b.deceleration = newConf.Deceleration

	// If we have an old comm object but it doesn't match the new config, shut it down and pretend
	// it never existed.
	if b.comm != nil && b.comm.GetUri() != newConf.URI {
		b.comm.Close()
		b.comm = nil // Create a new one next paragraph.
	}

	if b.comm == nil {
		if comm, err := getComm(b.cancelCtx, newConf, b.logger); err != nil {
			return err
		} else {
			b.comm = comm
		}
	}

	return nil
}

func getComm(ctx context.Context, conf *Config, logger golog.Logger) (CommPort, error) {
	switch {
	case strings.ToLower(conf.Protocol) == "can":
		// logger.Debug("Creating CAN Comm Port")
		return nil, fmt.Errorf("unsupported comm type %s", conf.Protocol)
		// return newCanComm(ctx, conf.URI, logger)
	case strings.ToLower(conf.Protocol) == "ip":
		logger.Debug("Creating IP Comm Port")
		if conf.ConnectTimeout == 0 {
			logger.Debug("Setting default connect timeout to 5 seconds")
			conf.ConnectTimeout = 5
		}
		timeout := time.Duration(conf.ConnectTimeout * int64(time.Second))
		return newIpComm(ctx, conf.URI, timeout, logger)
	case strings.ToLower(conf.Protocol) == "rs485":
		logger.Debug("Creating RS485 Comm Port")
		return newSerialComm(ctx, conf.URI, logger)
	case strings.ToLower(conf.Protocol) == "rs232":
		logger.Debug("Creating RS232 Comm Port")
		return newSerialComm(ctx, conf.URI, logger)
	default:
		return nil, fmt.Errorf("unknown comm type %s", conf.Protocol)
	}
}

func (s *ST) getStatus(ctx context.Context) ([]byte, error) {
	if resp, err := s.comm.Send(ctx, "SC"); err != nil {
		return nil, err
	} else {
		// TODO: document this better, once you've read the manual.

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
			if len(val) != 2 {
				return nil, ErrStatusMessageIncorrectLength
			}
			return val, nil
		}
	}
}

func isMoving(status []byte) (bool, error) {
	// TODO: document what status is
	if len(status) != 2 {
		return false, ErrStatusMessageIncorrectLength
	}
	return (status[1]>>4)&1 == 1, nil
}

func inPosition(status []byte) (bool, error) {
	if len(status) != 2 {
		return false, ErrStatusMessageIncorrectLength
	}
	return (status[1]>>3)&1 == 1, nil
}

func (s *ST) getBufferStatus(ctx context.Context) (int, error) {
	if resp, err := s.comm.Send(ctx, "BS"); err != nil {
		return -1, err
	} else {
		// TODO: document this better. The current comment doesn't match the code.
		// The response should look something like BS=<num>\r
		startIndex := strings.Index(resp, "=")
		if startIndex == -1 {
			return -1, fmt.Errorf("unable to find response data in %v", resp)
		}
		endIndex := strings.Index(resp, "{")
		if endIndex == -1 {
			endIndex = startIndex + 3
		}

		if endIndex > len(resp) {
			return 0, fmt.Errorf("unexpected response length %v", resp)
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
		case <-time.After(100 * time.Millisecond):
		}
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

func (s *ST) isBufferEmpty(ctx context.Context) (bool, error) {
	b, e := s.getBufferStatus(ctx)
	return b == 63, e
}

func (s *ST) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Debug("Closing comm port")
	return s.comm.Close()
}

func (s *ST) GoFor(ctx context.Context, rpm float64, positionRevolutions float64, extra map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger.Debugf("GoFor: rpm=%v, positionRevolutions=%v, extra=%v", rpm, positionRevolutions, extra)

	// Send the configuration commands to setup the motor for the move
	s.configureMove(ctx, positionRevolutions, rpm)

	// Then actually execute the move
	if _, err := s.comm.Send(ctx, "FL"); err != nil {
		// If the board errors here, is it wise to potentially just leave it running?
		return err
	}
	return s.waitForMoveCommandToComplete(ctx)
}

func (s *ST) GoTo(ctx context.Context, rpm float64, positionRevolutions float64, extra map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// FP?
	// For Ethernet drives, do not use FP with a position parameter. Instead, use DI to set the target position.
	// I guess this means run:
	// 	DI8000
	// 	FP
	s.logger.Debugf("GoTo: rpm=%v, positionRevolutions=%v, extra=%v", rpm, positionRevolutions, extra)
	// Send the configuration commands to setup the motor for the move
	s.configureMove(ctx, positionRevolutions, rpm)

	// Now execute the move command
	if _, err := s.comm.Send(ctx, "FP"); err != nil {
		return err
	}

	// Now wait for the command to finish
	return s.waitForMoveCommandToComplete(ctx)
}

func (s *ST) configureMove(ctx context.Context, positionRevolutions, rpm float64) error {
	// need to convert from RPM to revs per second
	revSec := rpm / 60
	// need to convert from revs to steps
	positionSteps := int64(positionRevolutions * float64(s.stepsPerRev))
	// Set the distance first
	if _, err := s.comm.Send(ctx, fmt.Sprintf("DI%d", positionSteps)); err != nil {
		return err
	}

	// Now set the velocity
	if _, err := s.comm.Send(ctx, fmt.Sprintf("VE%.4f", revSec)); err != nil {
		return err
	}

	// Set the acceleration, if we have it
	if s.acceleration > 0 {
		if _, err := s.comm.Send(ctx, fmt.Sprintf("AC%.3f", s.acceleration)); err != nil {
			return err
		}
	}
	// Set the deceleration, if we have it
	if s.deceleration > 0 {
		if _, err := s.comm.Send(ctx, fmt.Sprintf("DE%.3f", s.deceleration)); err != nil {
			return err
		}
	}
	return nil
}

func (s *ST) IsMoving(ctx context.Context) (bool, error) {
	s.logger.Debug("IsMoving forwarded to IsPowered")
	isMoving, _, err := s.IsPowered(ctx, nil)
	return isMoving, err
}

// IsPowered implements motor.Motor.
func (s *ST) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger.Debugf("IsPowered: extra=%v", extra)
	status, err := s.getStatus(ctx)
	if err != nil {
		return false, 0, err
	}
	isMoving, err := isMoving(status)
	return isMoving, 0, err
}

// Position implements motor.Motor.
func (s *ST) Position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger.Debugf("Position: extra=%v", extra)
	// EP?
	// IP?
	// The response should look something like IP=<num>\r
	if resp, err := s.comm.Send(ctx, "IP"); err != nil {
		return 0, err
	} else {
		startIndex := strings.Index(resp, "=")
		if startIndex == -1 {
			return 0, fmt.Errorf("unexpected response %v", resp)
		}
		endIndex := strings.Index(resp, "\r")
		if endIndex == -1 {
			return 0, fmt.Errorf("unexpected response %v", resp)
		}
		resp = resp[startIndex+1 : endIndex]
		if val, err := strconv.ParseUint(resp, 16, 32); err != nil {
			return 0, err
		} else {
			return float64(val), nil
		}
	}
}

// Properties implements motor.Motor.
func (s *ST) Properties(ctx context.Context, extra map[string]interface{}) (motor.Properties, error) {
	return motor.Properties{PositionReporting: true}, nil
}

// ResetZeroPosition implements motor.Motor.
func (s *ST) ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// EP0?
	// SP0?
	// The docs seem to indicate that for proper reset to 0, you must send both EP0 and SP0
	s.logger.Debugf("ResetZeroPosition: offset=%v", offset)
	// First reset the encoder
	if _, err := s.comm.Send(ctx, "EP0"); err != nil {
		return err
	}

	// Then reset the internal position
	if _, err := s.comm.Send(ctx, "SP0"); err != nil {
		return err
	}

	return nil
}

// SetPower implements motor.Motor.
func (s *ST) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return errors.New("set power is not supported for this motor")
	/*
		// VE? This is in rev/sec
		desiredRpm := s.maxRpm * powerPct
		s.logger.Warn("SetPower called on motor that uses rotational velocity. Scaling %v based on max Rpm %v. Resulting power: %v", powerPct, s.maxRpm, desiredRpm)

		// Send the configuration commands to setup the motor for the move
		s.configureMove(ctx, int64(math.MaxInt32), desiredRpm)

		// Now execute the move command
		if _, err := s.comm.Send(ctx, "FP"); err != nil {
			return err
		}
		// We explicitly don't want to wait for the command to finish
		return nil
	*/
}

// Stop implements motor.Motor.
func (s *ST) Stop(ctx context.Context, extras map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// SK - Stop & Kill? Stops and erases queue
	// SM - Stop Move? Stops and leaves queue intact?
	// ST - Halts the current buffered command being executed, but does not affect other buffered commands in the command buffer
	s.logger.Debugf("Stop called with %v", extras)
	_, err := s.comm.Send(ctx, "SC")
	if err != nil {
		return err
	}
	return nil
}

func (s *ST) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger.Debug("DoCommand called with %v", cmd)
	command := cmd["command"].(string)
	response, err := s.comm.Send(ctx, command)
	return map[string]interface{}{"response": response}, err
}
