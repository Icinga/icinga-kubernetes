package notifications

import (
	"context"
	"fmt"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	"github.com/pkg/errors"
)

// SyncSourceConfig synchronises the Icinga Notifications credentials from the YAML config to the database.
func SyncSourceConfig(ctx context.Context, db *database.Database, config *Config) error {
	var configPairs []*schemav1.Config

	if config.Url != "" {
		configPairs = []*schemav1.Config{
			{Key: schemav1.ConfigKeyNotificationsUrl, Value: config.Url},
			{Key: schemav1.ConfigKeyNotificationsKubernetesWebUrl, Value: config.KubernetesWebUrl},
			{Key: schemav1.ConfigKeyNotificationsUsername, Value: config.Username},
			{Key: schemav1.ConfigKeyNotificationsPassword, Value: config.Password},
			{Key: schemav1.ConfigKeyNotificationsLocked, Value: "true"},
		}

		stmt := fmt.Sprintf(
			`DELETE FROM %s WHERE %s IN (?)`,
			db.QuoteIdentifier(database.TableName(&schemav1.Config{})),
			"`key`",
		)

		if _, err := db.ExecContext(ctx, stmt, schemav1.ConfigKeyNotificationsSourceID); err != nil {
			return errors.Wrap(err, "cannot delete Icinga Notifications credentials")
		}
	} else {
		configPairs = []*schemav1.Config{
			{Key: schemav1.ConfigKeyNotificationsLocked, Value: "false"},
		}
	}

	stmt, _ := db.BuildUpsertStmt(&schemav1.Config{})
	if _, err := db.NamedExecContext(ctx, stmt, configPairs); err != nil {
		return errors.Wrap(err, "cannot upsert Icinga Notifications credentials")
	}

	return nil
}

// RetrieveConfig retrieves the Icinga Notifications config from the database. The username is "source-<sourceID>".
func RetrieveConfig(ctx context.Context, db *database.Database, config *Config) error {
	var dbConfig []*schemav1.Config
	if err := db.SelectContext(ctx, &dbConfig, db.BuildSelectStmt(&schemav1.Config{}, &schemav1.Config{})); err != nil {
		return errors.Wrap(err, "cannot fetch Icinga Notifications config from DB")
	}

	var locked bool

	for _, pair := range dbConfig {
		if pair.Key == schemav1.ConfigKeyNotificationsLocked {
			if pair.Value == "true" {
				locked = true
			} else {
				locked = false
			}
		}
	}

	for _, pair := range dbConfig {
		switch pair.Key {
		case schemav1.ConfigKeyNotificationsUrl:
			config.Url = pair.Value
		case schemav1.ConfigKeyNotificationsKubernetesWebUrl:
			config.KubernetesWebUrl = pair.Value
		case schemav1.ConfigKeyNotificationsPassword:
			config.Password = pair.Value
		case schemav1.ConfigKeyNotificationsSourceID:
			if !locked {
				config.Username = "source-" + pair.Value
			}
		case schemav1.ConfigKeyNotificationsUsername:
			if locked {
				config.Username = pair.Value
			}
		}
	}

	return nil
}
