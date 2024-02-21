package st

import (
	"errors"

	"go.viam.com/rdk/utils"
)

type config struct {
	Attributes     utils.AttributeMap `json:"attributes,omitempty"`
	Protocol       string             `json:"protocol"`
	URI            string             `json:"uri"`
	MinRpm         float64            `json:"min_rpm"`
	MaxRpm         float64            `json:"max_rpm"`
	StepsPerRev    int64              `json:"steps_per_rev"`
	ConnectTimeout int64              `json:"connect_timeout,omitempty"`
	Acceleration   float64            `json:"acceleration,omitempty"`
	Deceleration   float64            `json:"deceleration,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (conf *config) Validate(path string) ([]string, error) {
	if conf.Protocol == "" {
		return nil, errors.New("protocol is required")
	}
	if conf.URI == "" {
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
