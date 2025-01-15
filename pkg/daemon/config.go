package daemon

import (
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-kubernetes/pkg/metrics"
	"github.com/icinga/icinga-kubernetes/pkg/notifications"
)

// DefaultConfigPath specifies the default location of Icinga DB's config.yml for package installations.
// const DefaultConfigPath = "/etc/icinga-kubernetes/config.yml"
const DefaultConfigPath = "./config.yml"

// Config defines Icinga Kubernetes config.
type Config struct {
	Database      database.Config          `yaml:"database" envPrefix:"DATABASE_"`
	Logging       logging.Config           `yaml:"logging" envPrefix:"LOGGING_"`
	Notifications notifications.Config     `yaml:"notifications" envPrefix:"NOTIFICATIONS_"`
	Prometheus    metrics.PrometheusConfig `yaml:"prometheus" envPrefix:"PROMETHEUS_"`
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

	return c.Notifications.Validate()
}

// ConfigFlagGlue provides a glue struct for the CLI config flag.
type ConfigFlagGlue struct {
	// Config is the path to the config file
	Config string
}

// GetConfigPath retrieves the path to the configuration file.
// It returns the path specified via the command line, or DefaultConfigPath if none is provided.
func (f ConfigFlagGlue) GetConfigPath() string {
	if f.Config == "" {
		return DefaultConfigPath
	}

	return f.Config
}

// IsExplicitConfigPath indicates whether the configuration file path was explicitly set.
func (f ConfigFlagGlue) IsExplicitConfigPath() bool {
	return f.Config != ""
}
