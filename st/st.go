package st

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/resource"
)

var Model = resource.NewModel("viam-labs", "appliedmotion", "st")

type st struct {
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
		motor.API,
		Model,
		resource.Registration[motor.Motor, *config]{Constructor: NewMotor})
}

func NewMotor(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (motor.Motor, error) {
	logger.Info("Starting Applied Motion Products ST Motor Driver v0.1")
	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	s := st{
		Named:      conf.ResourceName().AsNamed(),
		logger:     logger,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		mu:         sync.RWMutex{},
	}

	if err := s.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return &s, nil
}

func (s *st) Reconfigure(ctx context.Context, _ resource.Dependencies, conf resource.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger.Debug("Reconfiguring Applied Motion Products ST Motor Driver")

	newConf, err := resource.NativeConfig[*config](conf)
	if err != nil {
		return err
	}

	// In case the module has changed name
	s.Named = conf.ResourceName().AsNamed()

	// Update the min/max RPM
	s.minRpm = newConf.MinRpm
	s.maxRpm = newConf.MaxRpm

	// Update the steps per rev
	s.stepsPerRev = newConf.StepsPerRev

	// If we have an old comm object, shut it down. We'll set it up again next paragraph.
	if s.comm != nil {
		s.comm.Close()
		s.comm = nil
	}

	if comm, err := getComm(s.cancelCtx, newConf, s.logger); err != nil {
		return err
	} else {
		s.comm = comm
	}

	s.acceleration = newConf.Acceleration
	if s.acceleration > 0 {
		if _, err := s.comm.Send(ctx, fmt.Sprintf("AC%.3f", s.acceleration)); err != nil {
			return err
		}
	}

	s.deceleration = newConf.Deceleration
	if s.deceleration > 0 {
		if _, err := s.comm.Send(ctx, fmt.Sprintf("DE%.3f", s.deceleration)); err != nil {
			return err
		}
	}
	// Set the maximum deceleration when stopping a move in the middle, too.
	stopDecel := math.Max(s.acceleration, s.deceleration)
	if stopDecel > 0 {
		if _, err := s.comm.Send(ctx, fmt.Sprintf("AM%.3f", stopDecel)); err != nil {
			return err
		}
	}

	return nil
}

func getComm(ctx context.Context, conf *config, logger golog.Logger) (CommPort, error) {
	switch {
	case strings.ToLower(conf.Protocol) == "can":
		return nil, fmt.Errorf("unsupported comm type %s", conf.Protocol)
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

func (s *st) getStatus(ctx context.Context) ([]byte, error) {
	if resp, err := s.comm.Send(ctx, "SC"); err != nil {
		return nil, err
	} else {
		// TODO: document this better, once you've read the manual.

		// Response format: "SC=0009{63"
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

func inPosition(status []byte) (bool, error) {
	if len(status) != 2 {
		return false, ErrStatusMessageIncorrectLength
	}
	return (status[1]>>3)&1 == 1, nil
}

func (s *st) getBufferStatus(ctx context.Context) (int, error) {
	if resp, err := s.comm.Send(ctx, "BS"); err != nil {
		return -1, err
	} else {
		// TODO: document this better. The current comment doesn't match the code.
		// The response should look something like BS=<num>
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

func (s *st) waitForMoveCommandToComplete(ctx context.Context) error {
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

func (s *st) isBufferEmpty(ctx context.Context) (bool, error) {
	b, e := s.getBufferStatus(ctx)
	return b == 63, e
}

func (s *st) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Debug("Closing comm port")
	return s.comm.Close()
}

func (s *st) GoFor(ctx context.Context, rpm float64, positionRevolutions float64, extra map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger.Debugf("GoFor: rpm=%v, positionRevolutions=%v, extra=%v", rpm, positionRevolutions, extra)

	oldAcceleration, err := SetOverrides(ctx, s.comm, extra)
	if err != nil {
		return err
	}

	// Send the configuration commands to setup the motor for the move
	s.configureMove(ctx, positionRevolutions, rpm)

	// Then actually execute the move
	if _, err := s.comm.Send(ctx, "FL"); err != nil {
		return err
	}
	return multierr.Combine(s.waitForMoveCommandToComplete(ctx),
	                        oldAcceleration.Restore(ctx, s.comm))
}

func (s *st) GoTo(ctx context.Context, rpm float64, positionRevolutions float64, extra map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// FP?
	// For Ethernet drives, do not use FP with a position parameter. Instead, use DI to set the target position.
	// I guess this means run:
	// 	DI8000
	// 	FP
	s.logger.Debugf("GoTo: rpm=%v, positionRevolutions=%v, extra=%v", rpm, positionRevolutions, extra)

	oldAcceleration, err := SetOverrides(ctx, s.comm, extra)
	if err != nil {
		return err
	}

	// Send the configuration commands to setup the motor for the move
	s.configureMove(ctx, positionRevolutions, rpm)

	// Now execute the move command
	if _, err := s.comm.Send(ctx, "FP"); err != nil {
		return err
	}
	return multierr.Combine(s.waitForMoveCommandToComplete(ctx),
	                        oldAcceleration.Restore(ctx, s.comm))
}

func (s *st) configureMove(ctx context.Context, positionRevolutions, rpm float64) error {
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

	return nil
}

func (s *st) IsMoving(ctx context.Context) (bool, error) {
	// If we locked the mutex, we'd block until after any GoFor or GoTo commands were finished! We
	// also aren't mutating any state in the struct itself, so there is no need to lock it.
	s.logger.Debug("IsMoving")
	status, err := s.getStatus(ctx)

	if err != nil {
		return false, err
	}
	if len(status) != 2 {
		return false, ErrStatusMessageIncorrectLength
	}
	return (status[1]>>4)&1 == 1, nil
}

// IsPowered implements motor.Motor.
func (s *st) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
	// The same as IsMoving, don't lock the mutex.
	s.logger.Debugf("IsPowered: extra=%v", extra)
	status, err := s.getStatus(ctx)
	if err != nil {
		return false, 0, err
	}
	if len(status) != 2 {
		return false, 0, ErrStatusMessageIncorrectLength
	}
	// The second return value is supposed to be the fraction of power sent to the motor, between 0
	// (off) and 1 (maximum power). It's unclear how to implement this for a stepper motor, so we
	// return 0 no matter what.
	return (status[1] & 1 == 1), 0, err
}

// Position implements motor.Motor.
func (s *st) Position(ctx context.Context, extra map[string]interface{}) (float64, error) {
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
		resp = resp[startIndex+1:]
		if val, err := strconv.ParseUint(resp, 16, 32); err != nil {
			return 0, err
		} else {
			return float64(val), nil
		}
	}
}

// Properties implements motor.Motor.
func (s *st) Properties(ctx context.Context, extra map[string]interface{}) (motor.Properties, error) {
	return motor.Properties{PositionReporting: true}, nil
}

// ResetZeroPosition implements motor.Motor.
func (s *st) ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
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
func (s *st) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	// We could tell it to move at a certain speed for a very large number of rotations, but that's
	// as close as this motor gets to having a "set power" function. A sketch of that
	// implementation is commented out below.
	return errors.New("set power is not supported for this motor")
	/*
		s.mu.Lock()
		defer s.mu.Unlock()

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
func (s *st) Stop(ctx context.Context, extras map[string]interface{}) error {
	// SK - Stop & Kill? Stops and erases queue
	// SM - Stop Move? Stops and leaves queue intact?
	// ST - Halts the current buffered command being executed, but does not affect other buffered commands in the command buffer
	s.logger.Debugf("Stop called with %v", extras)
	_, err := s.comm.Send(ctx, "SK") // Stop the current move and clear any queued moves, too.
	if err != nil {
		return err
	}
	return nil
}

func (s *st) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger.Debug("DoCommand called with %v", cmd)
	command := cmd["command"].(string)
	response, err := s.comm.Send(ctx, command)
	return map[string]interface{}{"response": response}, err
}
