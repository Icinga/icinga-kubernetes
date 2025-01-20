package metrics

import (
	"github.com/pkg/errors"
)

// PrometheusConfig defines Prometheus configuration.
type PrometheusConfig struct {
	Url      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// Validate checks constraints in the supplied Prometheus configuration and returns an error if they are violated.
func (c *PrometheusConfig) Validate() error {
	if c.Url != "" && (c.Username == "") != (c.Password == "") {
		return errors.New("both username and password must be provided")
	}

	return nil
}
