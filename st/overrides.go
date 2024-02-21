package st

import (
	"context"
	"fmt"

	"go.uber.org/multierr"
)

type oldAcceleration struct {
	acceleration string
	deceleration string
	// Perhaps more parameters will go here.
}

func setOverrides(
	ctx context.Context, comms commPort, extra map[string]interface{},
) (oldAcceleration, error) {
	var err error

	// This function does the heavy lifting of writing to the device and updating err. It returns
	// values to put into the old state.
	store := func (key, command string) string {
		val, exists := extra[key]
		if !exists {
			return "" // Use the default
		}

		realVal, ok := val.(float32)
		if !ok {
			err = multierr.Combine(err, fmt.Errorf("malformed value for %s: %#v", key, val))
			return ""
		}
		response, sendErr := replaceValue(ctx, comms, fmt.Sprintf("%s%.3f", command, realVal))
		err = multierr.Combine(err, sendErr)
		if response[:3] != command + "=" {
			// The response we got back does not match the request we sent (e.g., we sent an "AC"
			// request but did not get an "AC=" response). Something has gone very wrong.
			err = multierr.Combine(err, fmt.Errorf("unexpected response when storing %s: %#v",
												   key, response))
			return ""
		}
		return response[3:]
	}

	var os oldAcceleration
	os.acceleration = store("acceleration", "AC")
	os.deceleration = store("deceleration", "DE")
	return os, err
}

func (os *oldAcceleration) restore(ctx context.Context, comms commPort) error {
	// This function does all the heavy lifting of restoring the old state.
	restore := func (command, value string) error {
		if value == "" {
			return nil // No old state stored
		}
		_, err := comms.Send(ctx, command + value)
		return err
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
func replaceValue(ctx context.Context, s commPort, command string) (string, error) {
	response, err := s.Send(ctx, command[:2])
	if err != nil {
		return "", err
	}
	if _, err := s.Send(ctx, command); err != nil {
		return "", err
	}
	return response, nil
}
