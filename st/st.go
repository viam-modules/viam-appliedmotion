package st

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
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
	mu                      sync.RWMutex
	logger                  golog.Logger
	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
	comm                    SendCloser
	props                   motor.Properties
	minRpm                  float32
	maxRpm                  float32
}

func init() {
	resource.RegisterComponent(
		board.API,
		Model,
		resource.Registration[motor.Motor, *Config]{Constructor: newMotor})
}

func newMotor(ctx context.Context, _ resource.Dependencies, conf resource.Config, logger golog.Logger) (motor.Motor, error) {
	logger.Info("Starting Applied Motion Products ST-IP Driver v0.1")
	actualConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}
	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	comm, err := getComm(actualConf, cancelCtx)
	if err != nil {
		// Is this right?
		defer cancelFunc()
		return nil, err
	}

	motorProps := motor.Properties{
		PositionReporting: true,
	}
	b := ST{
		Named:      conf.ResourceName().AsNamed(),
		logger:     logger,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		mu:         sync.RWMutex{},
		minRpm:     actualConf.MinRpm,
		maxRpm:     actualConf.MaxRpm,
		props:      motorProps,
		comm:       comm,
	}

	if err := b.Reconfigure(ctx, nil, conf); err != nil {
		return nil, err
	}
	return &b, nil
}

func getComm(conf *Config, ctx context.Context) (SendCloser, error) {
	if strings.ToLower(conf.Protocol) == "ip" {
		timeout := time.Duration(conf.ConnectTimeout * int64(time.Second))
		return newIpComm(ctx, conf.IpAddress, timeout)
	}
	return nil, fmt.Errorf("unknown comm type %s", conf.Protocol)
}

func (b *ST) Reconfigure(
	ctx context.Context,
	_ resource.Dependencies,
	conf resource.Config,
) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	_, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}
	return nil
}

func (s *ST) Close(ctx context.Context) error {
	return s.comm.Close()
}

func (s *ST) GoFor(ctx context.Context, rpm float64, positionRevolutions float64, extra map[string]interface{}) error {
	// FL
	_, err := s.comm.Send(ctx, fmt.Sprintf("DI%v", positionRevolutions))
	if err != nil {
		return err
	}
	_, err = s.comm.Send(ctx, "FL")
	if err != nil {
		return err
	}
	return nil
}
func (s *ST) GoTo(ctx context.Context, rpm float64, positionRevolutions float64, extra map[string]interface{}) error {
	// FP?
	// For Ethernet drives, do not use FP with a position parameter. Instead, use DI to set the target position.
	// I guess this means run:
	// 	DI8000
	// 	FP
	_, err := s.comm.Send(ctx, fmt.Sprintf("DI%v", positionRevolutions))
	if err != nil {
		return err
	}
	_, err = s.comm.Send(ctx, "FP")
	if err != nil {
		return err
	}
	return nil
}

func (s *ST) IsMoving(ctx context.Context) (bool, error) {
	// SC - 0x0010
	resp, err := s.comm.Send(ctx, "SC")
	if err != nil {
		return false, err
	}
	val, err := hex.DecodeString(resp)
	if err != nil {
		return false, err
	}
	if len(val) > 4 {
		return false, errors.New("unexpected status length")
	}
	return (val[1]>>5)&1 == 1, nil
}

// IsPowered implements motor.Motor.
func (s *ST) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
	// SC - 0x0010
	resp, err := s.comm.Send(ctx, "SC")
	if err != nil {
		return false, 0, err
	}
	val, err := hex.DecodeString(resp)
	if err != nil {
		return false, 0, err
	}
	if len(val) > 4 {
		return false, 0, errors.New("unexpected status length")
	}
	return (val[1]>>5)&1 == 1, 0, nil
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
	// VE?
	return nil
}

// Stop implements motor.Motor.
func (s *ST) Stop(context.Context, map[string]interface{}) error {
	// SK - Stop & Kill? Stops and erases queue
	// SM - Stop Move? Stops and leaves queue intact?
	// ST - Halts the current buffered command being executed, but does not affect other buffered commands in the command buffer
	return nil
}
