package notifications

import (
	"net/url"

	"github.com/pkg/errors"
)

type Config struct {
	// If URL is the empty string, notifications are disabled.
	Url              string `yaml:"url" env:"URL"`
	Username         string `yaml:"username" env:"USERNAME"`
	Password         string `yaml:"password" env:"PASSWORD"`
	KubernetesWebUrl string `yaml:"kubernetes_web_url" env:"KUBERNETES_WEB_URL" default:"http://localhost/icingaweb2/kubernetes"`
}

// Validate checks constraints in the supplied configuration and returns an error if they are violated.
func (c *Config) Validate() error {
	if c.Url != "" || c.Username != "" || c.Password != "" {
		if c.Url == "" || c.Username == "" || c.Password == "" {
			return errors.New("if one of 'url', 'username', or 'password' is set, all must be set")
		}

		if _, err := url.Parse(c.Url); err != nil {
			return errors.Wrap(err, "'url' invalid")
		}
	}

	if _, err := url.Parse(c.KubernetesWebUrl); err != nil {
		return errors.Wrap(err, "'kubernetes_web_url' invalid")
	}

	return nil
}
