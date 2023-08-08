package st

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/stretchr/testify/assert"
	"go.viam.com/rdk/resource"
)

func getMotorForTesting(t *testing.T) (context.Context, *ST, error) {
	ctx := context.TODO()
	logger := golog.NewTestLogger(t)
	logger.WithOptions()
	config := resource.Config{
		ConvertedAttributes: &Config{
			URI:            "10.10.10.10:7776",
			Protocol:       "ip",
			MinRpm:         0,
			MaxRpm:         900,
			ConnectTimeout: 30,
		},
	}
	m, e := NewMotor(ctx, nil, config, logger)

	// unwrap motor.Motor into ST so we can access some non-interface members
	st, _ := m.(*ST)
	return ctx, st, e
}

func TestMotorIsMoving(t *testing.T) {
	ctx, motor, err := getMotorForTesting(t)
	assert.Nil(t, err, "failed to construct motor")

	isMoving, err := motor.IsMoving(ctx)
	assert.Nil(t, err, "failed to get motor status")
	assert.False(t, isMoving, "motor should be stopped")

	err = motor.GoTo(ctx, 600, 10, nil)
	assert.Nil(t, err, "error executing move command")
}

func TestStatusFunctions(t *testing.T) {
	ctx, motor, err := getMotorForTesting(t)
	assert.Nil(t, err, "failed to construct motor")

	status, err := motor.getStatus(ctx)
	assert.Nil(t, err, "failed to get motor status")
	assert.True(t, inPosition(status), "expected motor to be in position, status %#v", status)
	assert.False(t, isMoving(status), "expected motor to be stopped, status %#v", status)

	bufferSize, err := motor.getBufferStatus(ctx)
	assert.Nil(t, err, "failed to get buffer status")
	assert.Equal(t, 63, bufferSize, "buffer is not empty")
}
