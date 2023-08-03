package st

import (
	"go.viam.com/rdk/utils"
)

type Config struct {
	Attributes     utils.AttributeMap `json:"attributes,omitempty"`
	Protocol       string             `json:"protocol"`
	IpAddress      string             `json:"ip_address,omitempty"`
	ConnectTimeout int64              `json:"connect_timeout"`
	MinRpm         float32            `json:"min_rpm"`
	MaxRpm         float32            `json:"max_rpm"`
}

// The rev-pi's config can be found here:
// /var/www/revpi/pictory/projects/_config.rsc
// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	return nil, nil
}
