package api

type Config struct {
	Log LogStreamApiConfig `yaml:"log"`
}

func (c *Config) Validate() error {
	if err := c.Log.Validate(); err != nil {
		return err
	}

	return nil
}
