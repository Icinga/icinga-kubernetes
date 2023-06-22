package database

import (
	"github.com/creasty/defaults"
	"github.com/goccy/go-yaml"
	"github.com/pkg/errors"
	"os"
)

func FromYAMLFile(file string) (*Config, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, errors.Wrap(err, "can't open YAML file "+file)
	}
	defer f.Close()

	c := &struct {
		Database Config `yaml:"database"`
	}{}
	decoder := yaml.NewDecoder(f)

	if err := defaults.Set(c); err != nil {
		return nil, errors.Wrap(err, "can't set config defaults")
	}

	if err := decoder.Decode(c); err != nil {
		return nil, errors.Wrap(err, "can't parse YAML file "+file)
	}

	if err := c.Database.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid configuration")
	}

	return &c.Database, nil
}
