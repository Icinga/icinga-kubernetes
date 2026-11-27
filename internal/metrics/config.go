package metrics

import (
	"github.com/pkg/errors"
)

// PrometheusConfig defines Prometheus configuration.
type PrometheusConfig struct {
	Url       string `yaml:"url" env:"URL"`
	Insecure  string `yaml:"insecure" env:"INSECURE"`
	Username  string `yaml:"username" env:"USERNAME"`
	Password  string `yaml:"password" env:"PASSWORD"`
	Token     string `yaml:"token" env:"TOKEN"`
	TokenFile string `yaml:"token_file" env:"TOKEN_FILE"`
}

// Validate checks constraints in the supplied Prometheus configuration and
// returns an error if they are violated.
func (c *PrometheusConfig) Validate() error {
	// Values loaded later from the database or by auto-detection never clear
	// local ones, so conflicting local authentication options stay conflicting
	// even without a local URL.
	if c.Username != "" && (c.Token != "" || c.TokenFile != "") {
		return errors.New("username/password and token authentication are mutually exclusive")
	}

	if c.Token != "" && c.TokenFile != "" {
		return errors.New("token and token_file are mutually exclusive")
	}

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
