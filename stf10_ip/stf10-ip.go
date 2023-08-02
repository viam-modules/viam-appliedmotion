package stf10_ip

import (
	"context"
	"net"
	"sync"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/resource"
)

var Model = resource.NewModel("thegreatco", "motor", "stf10-ip")

type sft10_ip struct {
	resource.Named
	mu                      sync.RWMutex
	logger                  golog.Logger
	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
	socket                  net.Conn
	mixRpm                  float32
	maxRpm                  float32
}

func init() {
	resource.RegisterComponent(
		board.API,
		Model,
		resource.Registration[motor.Motor, *Config]{Constructor: newMotor})
}

func newMotor(
	ctx context.Context,
	_ resource.Dependencies,
	conf resource.Config,
	logger golog.Logger,
) (motor.Motor, error) {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	logger.Info("Starting Applied Motion Products STF10-IP Driver v0.1")
	b := sft10_ip{
		Named:      conf.ResourceName().AsNamed(),
		logger:     logger,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}

	if err := b.Reconfigure(ctx, nil, conf); err != nil {
		return nil, err
	}
	return &b, nil
}

func (b *sft10_ip) Reconfigure(
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

// Close implements motor.Motor.
func (s *sft10_ip) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.socket.Close()
	return nil
}

// GoFor implements motor.Motor.
func (*sft10_ip) GoFor(ctx context.Context, rpm float64, revolutions float64, extra map[string]interface{}) error {
	panic("unimplemented")
}

// GoTo implements motor.Motor.
func (*sft10_ip) GoTo(ctx context.Context, rpm float64, positionRevolutions float64, extra map[string]interface{}) error {
	// FP?
	panic("unimplemented")
}

// IsMoving implements motor.Motor.
func (*sft10_ip) IsMoving(context.Context) (bool, error) {
	panic("unimplemented")
}

// IsPowered implements motor.Motor.
func (*sft10_ip) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
	panic("unimplemented")
}

// Position implements motor.Motor.
func (*sft10_ip) Position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	// EP?
	panic("unimplemented")
}

// Properties implements motor.Motor.
func (*sft10_ip) Properties(ctx context.Context, extra map[string]interface{}) (motor.Properties, error) {
	panic("unimplemented")
}

// ResetZeroPosition implements motor.Motor.
func (*sft10_ip) ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	// EP0?
	panic("unimplemented")
}

// SetPower implements motor.Motor.
func (*sft10_ip) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	// VE?
	panic("unimplemented")
}

// Stop implements motor.Motor.
func (*sft10_ip) Stop(context.Context, map[string]interface{}) error {
	// SK - Stop & Kill? Stops and erases queue
	// SM - Stop Move? Stops and leaves queue intact?
	panic("unimplemented")
}
