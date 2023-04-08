package database

import (
	"github.com/goccy/go-yaml"
	"github.com/pkg/errors"
	"os"
)

type Database struct {
	Type     string `yaml:"type"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Database string `yaml:"database"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

type Config struct {
	Database Database `yaml:"database"`
}

func FromYAMLFile(file string) (*Config, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, errors.Wrap(err, "can't open YAML file "+file)
	}
	defer f.Close()

	c := &Config{}
	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(c); err != nil {
		return nil, errors.Wrap(err, "can't parse YAML file "+file)
	}

	if err := c.Database.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid configuration")
	}

	return c, nil
}

func (d *Database) Validate() error {
	// Validate config

	return nil
}
