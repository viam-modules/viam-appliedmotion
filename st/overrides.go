package st

import (
	"context"
	"fmt"
	"strconv"

	"go.uber.org/multierr"
)

type oldAcceleration struct {
	acceleration float64
	deceleration float64
	// Perhaps more parameters will go here.
}

func setOverrides(
	ctx context.Context, comms commPort, extra map[string]interface{},
) (oldAcceleration, error) {
	var err error

	// This function does the heavy lifting of writing to the device and updating err. It returns
	// values to put into the old state.
	store := func (key, command string) float64 {
		val, exists := extra[key]
		if !exists {
			return 0.0 // Use the default
		}

		realVal, ok := val.(float64)
		if !ok {
			err = multierr.Combine(err, fmt.Errorf("non-float64 value for %s: %#v", key, val))
			return 0.0
		}
		response, sendErr := replaceValue(ctx, comms, command, realVal)
		err = multierr.Combine(err, sendErr)
		if response[:3] != command + "=" {
			// The response we got back does not match the request we sent (e.g., we sent an "AC"
			// request but did not get an "AC=" response). Something has gone very wrong.
			err = multierr.Combine(err, fmt.Errorf("unexpected response when storing %s: %#v",
												   key, response))
			return 0.0
		}

		oldValue, convErr := strconv.ParseFloat(response[3:], 64)
		if convErr != nil {
			err = multierr.Combine(err, convErr)
			return 0.0
		}
		return oldValue
	}

	var os oldAcceleration
	os.acceleration = store("acceleration", "AC")
	os.deceleration = store("deceleration", "DE")
	return os, err
}

func (os *oldAcceleration) restore(ctx context.Context, comms commPort) error {
	// This function does all the heavy lifting of restoring the old state.
	restore := func (command string, value float64) error {
		if value == 0.0 {
			return nil // No old state stored
		}
		return comms.store(ctx, command, value)
	}

	return multierr.Combine(
		restore("AC", os.acceleration),
		restore("DE", os.deceleration),
	)
}

// replaceValue first sends on the commPort a version of the command with no arguments, then the
// entire command, and returns what it received from the first one. It is intended to be used to
// temporarily override some state in the motor controller.
// Example use: ReplaceValue(s, "AC100") sets the acceleration to 100 revs/sec^2 and returns the
// previous acceleration value. Later, you can use that return value to restore the acceleration to
// its original setting.
func replaceValue(ctx context.Context, s commPort, command string, value float64) (string, error) {
	response, err := s.send(ctx, command)
	if err != nil {
		return "", err
	}
	if err := s.store(ctx, command, value); err != nil {
		return "", err
	}
	return response, nil
}
