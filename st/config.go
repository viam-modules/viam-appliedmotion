package st

import (
	"errors"

	"go.viam.com/rdk/utils"
)

type Config struct {
	Attributes utils.AttributeMap `json:"attributes,omitempty"`

	Protocol       string `json:"protocol"`
	Uri            string `json:"uri"`
	ConnectTimeout int64  `json:"connect_timeout,omitempty"`

	StepsPerRev int64 `json:"steps_per_rev"`

	MinRpm       float64 `json:"min_rpm"`
	MaxRpm       float64 `json:"max_rpm"`
	Acceleration float64 `json:"acceleration,omitempty"`
	Deceleration float64 `json:"deceleration,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	if conf.Protocol == "" {
		return nil, errors.New("protocol is required")
	}
	if conf.Uri == "" {
		return nil, errors.New("URI is required")
	}
	if conf.MinRpm < 0 {
		return nil, errors.New("min_rpm must be >= 0")
	}
	if conf.MaxRpm <= 0 {
		return nil, errors.New("max_rpm must be > 0")
	}
	if conf.MaxRpm < conf.MinRpm {
		return nil, errors.New("max_rpm must be >= min_rpm")
	}
	if conf.StepsPerRev <= 0 {
		return nil, errors.New("steps_per_rev must be > 0")
	}

	return nil, nil
}
