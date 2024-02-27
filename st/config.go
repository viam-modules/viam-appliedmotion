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
	DefaultAcceleration float64 `json:"acceleration,omitempty"`
	DefaultDeceleration float64 `json:"deceleration,omitempty"`
	MinAcceleration     float64 `json:"min_acceleration,omitempty"`
	MinDeceleration     float64 `json:"min_deceleration,omitempty"`
	MaxAcceleration     float64 `json:"max_acceleration,omitempty"`
	MaxDeceleration     float64 `json:"max_deceleration,omitempty"`
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
	if conf.MaxRpm <= 0 {
		return nil, errors.New("max_rpm must be > 0")
	}
	if conf.MaxRpm < conf.MinRpm {
		return nil, errors.New("max_rpm must be >= min_rpm")
	}

	// Acceleration checks: start with a helper function
	checkLessThan := func(a, b float64, accelPrefix, prefixA, prefixB string) error {
		if a == 0 || b == 0 {
			// One of these is not defined and will be the default. The default is always ok.
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
		checkLessThan(conf.DefaultAcceleration, conf.MaxAcceleration,     "ac", "",     "max_"),
		checkLessThan(conf.MinDeceleration,     conf.MaxDeceleration,     "de", "min_", "max_"),
		checkLessThan(conf.MinDeceleration,     conf.DefaultDeceleration, "de", "min_", ""),
		checkLessThan(conf.DefaultDeceleration, conf.MaxDeceleration,     "de", "",     "max_"),
		checkNonNegative(conf.DefaultAcceleration, "acceleration"),
		checkNonNegative(conf.DefaultDeceleration, "acceleration"),
		checkNonNegative(conf.MinAcceleration, "min_acceleration"),
		checkNonNegative(conf.MinDeceleration, "min_acceleration"),
		checkNonNegative(conf.MaxAcceleration, "max_deceleration"),
		checkNonNegative(conf.MaxDeceleration, "max_deceleration"),
	)
}
