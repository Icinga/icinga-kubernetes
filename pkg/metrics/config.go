package metrics

// PrometheusConfig defines Prometheus configuration.
type PrometheusConfig struct {
	Url string `yaml:"url"`
}

// Validate checks constraints in the supplied Prometheus configuration and returns an error if they are violated.
func (c *PrometheusConfig) Validate() error {
	return nil
}
