package metrics

import (
	"github.com/pkg/errors"
)

// PrometheusConfig defines Prometheus configuration.
type PrometheusConfig struct {
	Url      string `yaml:"url" env:"URL"`
	Insecure string `yaml:"insecure" env:"INSECURE"`
	Username string `yaml:"username" env:"USERNAME"`
	Password string `yaml:"password" env:"PASSWORD"`
}

// Validate checks constraints in the supplied Prometheus configuration and returns an error if they are violated.
func (c *PrometheusConfig) Validate() error {
	if c.Url != "" {
		if (c.Username == "") != (c.Password == "") {
			return errors.New("both username and password must be provided")
		}

		if c.Insecure != "" && c.Insecure != "true" && c.Insecure != "false" {
			return errors.New("'insecure' has to be 'true', 'false' or empty")
		}
	}

	return nil
}
