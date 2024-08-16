package notifications

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	"github.com/pkg/errors"
)

// SyncSourceConfig synchronises the Icinga Notifications credentials from the YAML config with the database.
func SyncSourceConfig(ctx context.Context, db *database.Database, config *Config) error {
	var typPtr *schemav1.Config

	if IsAutoCreationEnabled(config) {
		username, password, err := retrieveCredentials(ctx, db)
		if err != nil {
			return err
		}

		if username != "" && password != "" {
			return nil
		}

		configPairs := []*schemav1.Config{
			{Key: schemav1.ConfigKeyNotificationsUsername, Value: "Icinga for Kubernetes"},
			{Key: schemav1.ConfigKeyNotificationsPassword, Value: uuid.NewString()},
		}

		stmt, _ := db.BuildUpsertStmt(typPtr)
		if _, err := db.NamedExecContext(ctx, stmt, configPairs); err != nil {
			return errors.Wrap(err, "cannot upsert Icinga Notifications credentials")
		}
	} else {
		stmt := fmt.Sprintf(
			"DELETE FROM %s WHERE %[2]s = :password OR %[2]s = :username",
			db.QuoteIdentifier(database.TableName(typPtr)),
			db.QuoteIdentifier("key"))

		// We purposefully do not delete the schemav1.ConfigKeyNotificationsSourceID key as it is used by
		// Icinga Notifications Web to delete the actual notification source and afterwards it'll delete it as well.
		args := map[string]any{
			"password": schemav1.ConfigKeyNotificationsPassword,
			"username": schemav1.ConfigKeyNotificationsUsername,
		}
		if _, err := db.NamedExecContext(ctx, stmt, args); err != nil {
			return errors.Wrap(err, "cannot delete Icinga Notifications credentials")
		}
	}

	return nil
}

// retrieveCredentials retrieves the Icinga Notifications credentials from the database.
func retrieveCredentials(ctx context.Context, db *database.Database) (string, string, error) {
	var typPtr *schemav1.Config

	var dbConfig []*schemav1.Config
	if err := db.SelectContext(ctx, &dbConfig, db.BuildSelectStmt(typPtr, typPtr)); err != nil {
		return "", "", errors.Wrap(err, "cannot fetch Icinga Notifications credentials from DB")
	}

	var username, password string
	for _, pair := range dbConfig {
		switch pair.Key {
		case schemav1.ConfigKeyNotificationsPassword:
			password = pair.Value
		case schemav1.ConfigKeyNotificationsSourceID:
			username = "source-" + pair.Value
		}
	}

	return username, password, nil
}
