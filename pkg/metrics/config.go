package metrics

import "github.com/pkg/errors"

// PrometheusConfig defines Prometheus configuration.
type PrometheusConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// Validate checks constraints in the supplied Prometheus configuration and returns an error if they are violated.
func (c *PrometheusConfig) Validate() error {
	if c.Host == "" {
		return errors.New("Prometheus host missing")
	}

	return nil
}
