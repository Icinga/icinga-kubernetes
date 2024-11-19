package notifications

import (
	"github.com/pkg/errors"
	"net/url"
	"regexp"
)

type Config struct {
	// If URL is the empty string, notifications are disabled.
	Url              string `yaml:"url"`
	Username         string `yaml:"username"`
	Password         string `yaml:"password"`
	KubernetesWebUrl string `yaml:"kubernetes_web_url" default:"http://localhost/icingaweb2/kubernetes"`
}

// Validate checks constraints in the supplied configuration and returns an error if they are violated.
func (c *Config) Validate() error {
	if c.Url != "" || c.Username != "" || c.Password != "" {
		if c.Url == "" || c.Username == "" || c.Password == "" {
			return errors.New("if one of 'url', 'username', or 'password' is set, all must be set")
		}

		usernameValid, err := regexp.MatchString(`^source-\d+$`, c.Username)
		if err != nil {
			return errors.WithStack(err)
		}
		if !usernameValid {
			return errors.New("'username' must be of the form 'source-<source_id>'")
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
