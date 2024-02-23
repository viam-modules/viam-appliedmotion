package st

import (
	"errors"

	"go.viam.com/rdk/utils"
)

type config struct {
	attributes     utils.AttributeMap `json:"attributes,omitempty"`
	protocol       string             `json:"protocol"`
	uri            string             `json:"uri"`
	minRpm         float64            `json:"min_rpm"`
	maxRpm         float64            `json:"max_rpm"`
	stepsPerRev    int64              `json:"steps_per_rev"`
	connectTimeout int64              `json:"connect_timeout,omitempty"`
	acceleration   float64            `json:"acceleration,omitempty"`
	deceleration   float64            `json:"deceleration,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (conf *config) Validate(path string) ([]string, error) {
	if conf.protocol == "" {
		return nil, errors.New("protocol is required")
	}
	if conf.uri == "" {
		return nil, errors.New("URI is required")
	}
	if conf.minRpm < 0 {
		return nil, errors.New("min_rpm must be >= 0")
	}
	if conf.maxRpm <= 0 {
		return nil, errors.New("max_rpm must be > 0")
	}
	if conf.maxRpm < conf.minRpm {
		return nil, errors.New("max_rpm must be >= min_rpm")
	}
	if conf.stepsPerRev <= 0 {
		return nil, errors.New("steps_per_rev must be > 0")
	}

	return nil, nil
}
