package notifications

import (
	"github.com/pkg/errors"
	"net/url"
	"strings"
)

type Config struct {
	Url              string `yaml:"url"`
	Username         string `yaml:"username"`
	Password         string `yaml:"password"`
	KubernetesWebUrl string `yaml:"kubernetes_web_url" default:"http://localhost/icingaweb2/kubernetes"`
}

// Validate implements the config.Validator interface.
func (c *Config) Validate() error {
	if (c.Username == "") != (c.Password == "") {
		return errors.New("'username' must be set, if password is provided and vice versa")
	}
	if c.Username != "" {
		// Since Icinga Notifications does not yet support basic HTTP authentication with a simple user and password,
		// we have to use a static “username” consisting of `source-` and the actual source ID for the time being.
		// See https://github.com/Icinga/icinga-notifications/issues/227
		parts := strings.Split(c.Username, "-")
		if len(parts) != 2 || parts[0] != "source" {
			return errors.New("'username' must be of the form '<source>-<SourceID>'")
		}
	}
	if c.Url == "" && c.Username != "" {
		return errors.New("Icinga Notifications base 'url' must be provided, if username and password are set")
	}

	if _, err := url.Parse(c.KubernetesWebUrl); err != nil {
		return errors.Wrapf(err, "cannot parse Icinga for Kubernetes Web base URL: %q", c.KubernetesWebUrl)
	}

	return nil
}
