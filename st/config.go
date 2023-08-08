package st

import (
	"go.viam.com/rdk/utils"
)

type Config struct {
	Attributes     utils.AttributeMap `json:"attributes,omitempty"`
	Protocol       string             `json:"protocol"`
	URI            string             `json:"uri,omitempty"`
	ConnectTimeout int64              `json:"connect_timeout"`
	MinRpm         float64            `json:"min_rpm"`
	MaxRpm         float64            `json:"max_rpm"`
	StepsPerRev    int64              `json:"steps_per_rev"`
}

// The rev-pi's config can be found here:
// /var/www/revpi/pictory/projects/_config.rsc
// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	return nil, nil
}
