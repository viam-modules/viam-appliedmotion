package st

import (
	"errors"
	"fmt"

	"go.uber.org/multierr"
	"go.viam.com/rdk/utils"
)

type Config struct {
	Attributes utils.AttributeMap `json:"attributes,omitempty"`

	Protocol       string `json:"protocol"`
	Uri            string `json:"uri"`
	ConnectTimeout int64  `json:"connect_timeout,omitempty"`

	StepsPerRev int64 `json:"steps_per_rev"`

	MinRpm              float64 `json:"min_rpm"`
	MaxRpm              float64 `json:"max_rpm"`
	DefaultAcceleration float64 `json:"default_accel_revs_per_sec_squared,omitempty"`
	DefaultDeceleration float64 `json:"default_decel_revs_per_sec_squared,omitempty"`
	MinAcceleration     float64 `json:"min_accel_revs_per_sec_squared,omitempty"`
	MaxAcceleration     float64 `json:"max_accel_revs_per_sec_squared,omitempty"`
	MinDeceleration     float64 `json:"min_decel_revs_per_sec_squared,omitempty"`
	MaxDeceleration     float64 `json:"max_decel_revs_per_sec_squared,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	if conf.Protocol == "" {
		return nil, errors.New("protocol is required")
	}
	if conf.Uri == "" {
		return nil, errors.New("URI is required")
	}
	if conf.StepsPerRev <= 0 {
		return nil, errors.New("steps_per_rev must be > 0")
	}

	// RPM checks
	if conf.MinRpm < 0 {
		return nil, errors.New("min_rpm must be >= 0")
	}
	if conf.MaxRpm < 0 {
		return nil, errors.New("max_rpm must be >= 0")
	}
	if conf.MaxRpm != 0 && conf.MaxRpm < conf.MinRpm {
		return nil, errors.New("max_rpm must be >= min_rpm")
	}

	// Acceleration checks: start with a helper function
	checkLessThan := func(a, b float64, accelPrefix, prefixA, prefixB string) error {
		if a == 0 || b == 0 {
			// If a maximum or minimum limit is not defined, we won't use that limit, and
			// everything is okay. If a default value is not defined, we'll use the value already
			// stored within the motor controller.
			// WARNING: if we set the max/min acceleration (or deceleration) but not the default,
			// it's possible that the motor controller's pre-set default value falls outside of the
			// max/min we intend to use. We assume the pre-set values are reasonable, but can
			// change this implementation if necessary.
			return nil
		}
		if a > b {
			return fmt.Errorf("%s%sceleration must be <= %s%sceleration", prefixA, accelPrefix, prefixB, accelPrefix)
		}
		return nil
	}

	checkNonNegative := func(val float64, name string) error {
		if val < 0 {
			return fmt.Errorf("%s must be >= 0", name)
		}
		return nil
	}

	return nil, multierr.Combine(
		checkLessThan(conf.MinAcceleration,     conf.MaxAcceleration,     "ac", "min_", "max_"),
		checkLessThan(conf.MinAcceleration,     conf.DefaultAcceleration, "ac", "min_", ""),
		checkLessThan(conf.DefaultAcceleration, conf.MaxAcceleration,     "ac", "default_", "max_"),
		checkLessThan(conf.MinDeceleration,     conf.MaxDeceleration,     "de", "min_", "max_"),
		checkLessThan(conf.MinDeceleration,     conf.DefaultDeceleration, "de", "min_", "default_"),
		checkLessThan(conf.DefaultDeceleration, conf.MaxDeceleration,     "de", "default_", "max_"),
		checkNonNegative(conf.DefaultAcceleration, "accel"),
		checkNonNegative(conf.DefaultDeceleration, "decel"),
		checkNonNegative(conf.MinAcceleration, "min_accel"),
		checkNonNegative(conf.MaxAcceleration, "max_decel"),
		checkNonNegative(conf.MinDeceleration, "min_accel"),
		checkNonNegative(conf.MaxDeceleration, "max_decel"),
	)
}
