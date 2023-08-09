package st

import (
	"context"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/stretchr/testify/assert"
	"go.viam.com/rdk/resource"
)

var StepsPerRev = int64(20000)

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
			ConnectTimeout: 1,
			StepsPerRev:    StepsPerRev,
			Acceleration:   100,
			Deceleration:   100,
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
	go func() {
		err = motor.GoFor(ctx, 600, 10, nil)
		assert.Nil(t, err, "error executing move command")
	}()
	// Sleep a bit to let the motor get going
	time.Sleep(50 * time.Millisecond)

	isMoving, err = motor.IsMoving(ctx)
	assert.Nil(t, err, "failed to get motor status")
	assert.True(t, isMoving, "motor should be moving")
}

func TestStatusFunctions(t *testing.T) {
	ctx, motor, err := getMotorForTesting(t)
	assert.Nil(t, err, "failed to construct motor")

	status, err := motor.getStatus(ctx)
	assert.Nil(t, err, "failed to get motor status")
	inPosition, err := inPosition(status)
	assert.Nil(t, err, "failed to get in position from status")
	assert.True(t, inPosition, "expected motor to be in position, status %#v", status)
	isMoving, err := isMoving(status)
	assert.Nil(t, err, "failed to get is moving from status")
	assert.False(t, isMoving, "expected motor to be stopped, status %#v", status)

	bufferSize, err := motor.getBufferStatus(ctx)
	assert.Nil(t, err, "failed to get buffer status")
	assert.Equal(t, 63, bufferSize, "buffer is not empty")
}

func TestGoFor(t *testing.T) {
	ctx, motor, err := getMotorForTesting(t)
	assert.Nil(t, err, "failed to construct motor")

	err = motor.GoFor(ctx, 600, .001, nil)
	assert.Nil(t, err, "error executing move command")

	err = motor.GoFor(ctx, 600, -.001, nil)
	assert.Nil(t, err, "error executing move command")
}

func TestGoTo(t *testing.T) {
	ctx, motor, err := getMotorForTesting(t)
	assert.Nil(t, err, "failed to construct motor")

	// First reset the position to 0
	err = motor.ResetZeroPosition(ctx, 0, nil)
	assert.Nil(t, err, "error resetting position")

	err = motor.GoTo(ctx, 100, .001, nil)
	assert.Nil(t, err, "error executing move command")

	position, err := motor.Position(ctx, nil)
	assert.Nil(t, err, "error getting position")
	expectedSteps := float64(StepsPerRev) * .001
	assert.Equal(t, expectedSteps, position, "position should be equal to %v", expectedSteps)

	err = motor.GoTo(ctx, 100, .01, nil)
	assert.Nil(t, err, "error executing move command")

	position, err = motor.Position(ctx, nil)
	assert.Nil(t, err, "error getting position")
	expectedSteps = float64(StepsPerRev) * .01
	assert.Equal(t, expectedSteps, position, "position should be equal to %v", expectedSteps)
}

func TestPosition(t *testing.T) {
	ctx, motor, err := getMotorForTesting(t)
	assert.Nil(t, err, "failed to construct motor")

	// First reset the position to 0
	err = motor.ResetZeroPosition(ctx, 0, nil)
	assert.Nil(t, err, "error resetting position")

	position, err := motor.Position(ctx, nil)
	assert.Nil(t, err, "error getting position")
	assert.Equal(t, 0.0, position, "position should be 0")

	// Move the motor a bit
	err = motor.GoFor(ctx, 600, .01, nil)
	assert.Nil(t, err, "error executing move command")

	// Check the position again
	position, err = motor.Position(ctx, nil)
	assert.Nil(t, err, "error getting position")
	expectedSteps := float64(StepsPerRev) * .01
	assert.Equal(t, expectedSteps, position, "position should be equal to %v", expectedSteps)

	// Move the motor a bit, but this time, backwards
	err = motor.GoFor(ctx, 600, -.01, nil)
	assert.Nil(t, err, "error executing move command")

	// Check the position again
	position, err = motor.Position(ctx, nil)
	assert.Nil(t, err, "error getting position")
	expectedSteps = 0
	assert.Equal(t, expectedSteps, position, "position should be equal to %v", expectedSteps)
}

func TestDoCommand(t *testing.T) {
	ctx, motor, err := getMotorForTesting(t)
	assert.Nil(t, err, "failed to construct motor")
	_, err = motor.DoCommand(ctx, map[string]interface{}{"command": "DI20000"})
	assert.Nil(t, err, "error executing do command")
	_, err = motor.DoCommand(ctx, map[string]interface{}{"command": "VE1"})
	assert.Nil(t, err, "error executing do command")
	_, err = motor.DoCommand(ctx, map[string]interface{}{"command": "AC100"})
	assert.Nil(t, err, "error executing do command")
	_, err = motor.DoCommand(ctx, map[string]interface{}{"command": "DE100"})
	assert.Nil(t, err, "error executing do command")
	resp, err := motor.DoCommand(ctx, map[string]interface{}{"command": "FL"})
	assert.Nil(t, err, "error executing do command")
	assert.NotNil(t, resp["response"], "response should not be nil")
}
