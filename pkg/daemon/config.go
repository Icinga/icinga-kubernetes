package daemon

import (
	"slices"

	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-kubernetes/pkg/metrics"
	"github.com/icinga/icinga-kubernetes/pkg/notifications"
	"github.com/pkg/errors"
)

// DefaultConfigPath specifies the default location of Icinga for Kubernetes's
// config.yml if not set via command line flag.
const DefaultConfigPath = "./config.yml"

// Config defines Icinga Kubernetes config.
type Config struct {
	Database      database.Config          `yaml:"database" envPrefix:"DATABASE_"`
	Logging       LoggingConfig            `yaml:"logging" envPrefix:"LOGGING_"`
	Notifications notifications.Config     `yaml:"notifications" envPrefix:"NOTIFICATIONS_"`
	Prometheus    metrics.PrometheusConfig `yaml:"prometheus" envPrefix:"PROMETHEUS_"`
	Namespaces    NamespacesConfig         `yaml:"namespaces" envPrefix:"NAMESPACES_"`
}

type NamespacesConfig struct {
	Include []string `yaml:"include" env:"INCLUDE"`
	Exclude []string `yaml:"exclude" env:"EXCLUDE"`
}

func (c *NamespacesConfig) Validate() error {
	if len(c.Exclude) > 0 && len(c.Include) > 0 {
		return errors.New("exclude and include can not be set at the same time")
	}

	if slices.Contains(c.Include, "") {
		return errors.New("include namespace can not be empty")
	}

	if slices.Contains(c.Exclude, "") {
		return errors.New("exclude namespace can not be empty")
	}

	var found []string
	for _, namespace := range c.Include {
		if slices.Contains(found, namespace) {
			return errors.New("namespace can not be included twice")
		}
		found = append(found, namespace)
	}

	for _, namespace := range c.Exclude {
		if slices.Contains(found, namespace) {
			return errors.New("namespace can not be excluded twice")
		}
		found = append(found, namespace)
	}

	return nil
}

// Validate checks constraints in the supplied configuration and returns
// an error if they are violated.
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

	if err := c.Namespaces.Validate(); err != nil {
		return err
	}

	return c.Notifications.Validate()
}

// ConfigFlagGlue provides a glue struct for the CLI config flag.
//
// ConfigFlagGlue implements the [github.com/icinga/icinga-go-library/config.Flags] interface.
type ConfigFlagGlue struct {
	// Config is the path to the config file
	Config string
}

// GetConfigPath retrieves the path to the configuration file. It returns the
// path specified via the command line, or DefaultConfigPath if none is provided.
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

// LoggingConfig extends the icinga-go-library logging configuration with the
// verbosity of the Kubernetes client libraries.
type LoggingConfig struct {
	logging.Config `yaml:",inline"`

	// Kubernetes is the klog verbosity of the Kubernetes client libraries, from 0 to 9.
	// It does not affect the log output of Icinga for Kubernetes itself,
	// and the -v/--v command line flag takes precedence over it.
	Kubernetes int32 `yaml:"kubernetes" env:"KUBERNETES" default:"0"`
}

// Validate checks constraints in the supplied logging configuration and returns an error if they are violated.
func (c *LoggingConfig) Validate() error {
	if c.Kubernetes < 0 || c.Kubernetes > 9 {
		return errors.New("logging.kubernetes must be between 0 and 9")
	}

	return c.Config.Validate()
}
