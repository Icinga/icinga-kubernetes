package metrics

import (
	"context"
	"github.com/icinga/icinga-go-library/database"
	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	"github.com/pkg/errors"
)

// PrometheusConfig defines Prometheus configuration.
type PrometheusConfig struct {
	Url string `yaml:"url"`
}

// Validate checks constraints in the supplied Prometheus configuration and returns an error if they are violated.
func (c *PrometheusConfig) Validate() error {
	return nil
}

func SyncPrometheusConfig(ctx context.Context, db *database.DB, config *PrometheusConfig) error {
	var configPairs []*schemav1.Config

	if config.Url != "" {
		configPairs = append(configPairs, &schemav1.Config{Key: schemav1.ConfigKeyPrometheusUrl, Value: config.Url})
		configPairs = append(configPairs, &schemav1.Config{Key: schemav1.ConfigKeyPrometheusLocked, Value: "true"})
	} else {
		configPairs = append(configPairs, &schemav1.Config{Key: schemav1.ConfigKeyPrometheusLocked, Value: "false"})
	}

	stmt, _ := db.BuildUpsertStmt(&schemav1.Config{})
	if _, err := db.NamedExecContext(ctx, stmt, configPairs); err != nil {
		return errors.Wrap(err, "cannot upsert Prometheus configuration")
	}

	return nil
}

func RetrievePrometheusConfig(ctx context.Context, db *database.DB, config *PrometheusConfig) error {
	var dbConfig []*schemav1.Config
	if err := db.SelectContext(ctx, &dbConfig, db.BuildSelectStmt(&schemav1.Config{}, &schemav1.Config{})); err != nil {
		return errors.Wrap(err, "cannot retrieve Prometheus configuration")
	}

	for _, c := range dbConfig {
		switch c.Key {
		case schemav1.ConfigKeyPrometheusUrl:
			config.Url = c.Value
		}
	}

	return nil
}
