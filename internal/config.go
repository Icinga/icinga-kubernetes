package internal

import (
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/pkg/errors"
)

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

// Config defines Icinga Kubernetes config.
type Config struct {
	Database   database.Config  `yaml:"database"`
	Logging    logging.Config   `yaml:"logging"`
	Prometheus PrometheusConfig `yaml:"prometheus"`
}

// Validate checks constraints in the supplied configuration and returns an error if they are violated.
func (c *Config) Validate() error {
	if err := c.Database.Validate(); err != nil {
		return err
	}

	if err := c.Logging.Validate(); err != nil {
		return err
	}

	if err := c.Prometheus.Validate(); err != nil {
		return err
	}

	return nil
}
