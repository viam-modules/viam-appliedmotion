package st

// NOTE: this test uses actual hardware on your local network! It does not use mocked-out hardware,
// and will fail if you don't have a motor controller at the IP address 10.10.10.10.

import (
	"context"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/stretchr/testify/assert"
	"go.viam.com/rdk/resource"
)

const stepsPerRev = 20000

func getMotorForTesting(t *testing.T) (context.Context, *st, error) {
	ctx := context.TODO()
	logger := golog.NewTestLogger(t)
	logger.WithOptions()
	config := resource.Config{
		ConvertedAttributes: &Config{
			Uri:            "10.10.10.10:7776",
			Protocol:       "ip",
			MinRpm:         0,
			MaxRpm:         900,
			ConnectTimeout: 1,
			StepsPerRev:    stepsPerRev,
			Acceleration:   100,
			Deceleration:   100,
		},
	}
	m, e := newMotor(ctx, nil, config, logger)

	// unwrap motor.Motor into st so we can access some non-interface members
	st, _ := m.(*st)
	return ctx, st, e
}

func TestMotorIsMoving(t *testing.T) {
	ctx, motor, err := getMotorForTesting(t)
	assert.Nil(t, err, "failed to construct motor")

	isMoving, err := motor.IsMoving(ctx)
	assert.Nil(t, err, "failed to get motor status")
	assert.False(t, isMoving, "motor should be stopped")
	done := make(chan bool)
	go func() {
		err = motor.GoFor(ctx, 600, 10, nil)
		assert.Nil(t, err, "error executing move command")
		close(done)
	}()
	// Sleep a bit to let the motor get going
	time.Sleep(50 * time.Millisecond)

	isMoving, err = motor.IsMoving(ctx)
	assert.Nil(t, err, "failed to get motor status")
	assert.True(t, isMoving, "motor should be moving")

	<-done // Wait for the motor to stop before going to the next test.
}

func TestStatusFunctions(t *testing.T) {
	ctx, motor, err := getMotorForTesting(t)
	assert.Nil(t, err, "failed to construct motor")

	status, err := motor.getStatus(ctx)
	assert.Nil(t, err, "failed to get motor status")
	inPosition, err := inPosition(status)
	assert.Nil(t, err, "failed to get in position from status")
	assert.True(t, inPosition, "expected motor to be in position, status %#v", status)
	isMoving, err := motor.IsMoving(ctx)
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
	expectedSteps := .001
	assert.Equal(t, expectedSteps, position, "position should be equal to %v", expectedSteps)

	err = motor.GoTo(ctx, 100, .01, nil)
	assert.Nil(t, err, "error executing move command")

	position, err = motor.Position(ctx, nil)
	assert.Nil(t, err, "error getting position")
	expectedSteps = .01
	assert.Equal(t, expectedSteps, position, "position should be equal to %v", expectedSteps)
}

func TestPosition(t *testing.T) {
	distance := 0.1 // revolutions to travel
	ctx, motor, err := getMotorForTesting(t)
	assert.Nil(t, err, "failed to construct motor")

	// First reset the position to 0
	err = motor.ResetZeroPosition(ctx, 0, nil)
	assert.Nil(t, err, "error resetting position")

	position, err := motor.Position(ctx, nil)
	assert.Nil(t, err, "error getting position")
	assert.Equal(t, 0.0, position, "position should be 0")

	// Move the motor a bit
	err = motor.GoFor(ctx, 600, distance, nil)
	assert.Nil(t, err, "error executing move command")

	// Check the position again
	position, err = motor.Position(ctx, nil)
	assert.Nil(t, err, "error getting position")
	assert.Equal(t, distance, position)

	// Move the motor a bit, but this time, backwards
	err = motor.GoFor(ctx, 600, -distance, nil)
	assert.Nil(t, err, "error executing move command")

	// Check the position again
	position, err = motor.Position(ctx, nil)
	assert.Nil(t, err, "error getting position")
	assert.Equal(t, 0.0, position)

	// Reset the position to a nonzero value
	err = motor.ResetZeroPosition(ctx, 1, nil)
	assert.Nil(t, err, "error resetting position")

	position, err = motor.Position(ctx, nil)
	assert.Nil(t, err, "error getting position")
	assert.Equal(t, -1.0, position)

	err = motor.GoFor(ctx, 600, 1, nil)
	assert.Nil(t, err, "error executing move command")

	position, err = motor.Position(ctx, nil)
	assert.Nil(t, err, "error getting position")
	assert.Equal(t, 0.0, position)
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

func TestAccelOverrides(t *testing.T) {
	ctx, motor, err := getMotorForTesting(t)
	assert.Nil(t, err, "failed to construct motor")

	// Since we're moving a real motor, we can use real time to see how fast it's going.
	t1 := time.Now()
	err = motor.GoFor(ctx, 600, 5, nil)
	assert.Nil(t, err, "error moving motor at default acceleration")
	t2 := time.Now()
	err = motor.GoFor(ctx, 600, 5, map[string]interface{}{"acceleration": 10.0})
	assert.Nil(t, err, "error moving motor at slower acceleration")
	t3 := time.Now()
	err = motor.GoFor(ctx, 600, 5, map[string]interface{}{"deceleration": 10.0})
	assert.Nil(t, err, "error moving motor at slower deceleration")
	t4 := time.Now()

	assert.Greater(t, t3.Sub(t2), 2 * t1.Sub(t2)) // Slow acceleration takes longer than default
	assert.Greater(t, t4.Sub(t3), 2 * t1.Sub(t2)) // Slow deceleration takes longer than default, too
}
