package st

import (
	"context"
)

// ReplaceValue first sends on the CommPort a version of the command with no arguments, then the
// entire command, and returns what it received from the first one. It is intended to be used to
// temporarily override some state in the motor controller.
// Example use: ReplaceValue(s, "AC100") sets the acceleration to 100 revs/sec^2 and returns the
// previous acceleration value. Later, you can use that return value to restore the acceleration to
// its original setting.
func ReplaceValue(ctx context.Context, s CommPort, command string) (string, error) {
	response, err := s.Send(ctx, command[:2])
	if err != nil {
		return "", err
	}
	if _, err := s.Send(ctx, command); err != nil {
		return "", err
	}
	return response, nil
}
