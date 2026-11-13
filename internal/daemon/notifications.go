package daemon

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"

	baseConfig "github.com/icinga/icinga-go-library/config"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/notifications/source"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/internal/schema"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

// SyncNotificationsConfig reconciles the Icinga Notifications source config with
// the "config" table for the given cluster and writes the result back into config.
//
// Values supplied via config file or environment are persisted as locked rows,
// which Icinga for Kubernetes Web treats as read-only. Missing values are filled
// in from unlocked rows, i.e. those configured through the web interface.
// Supplied values always take precedence. On every call, all previously locked
// notifications.* rows are replaced, so removing a value from the config also
// removes it from the database.
//
// If config.Url is set, only the URL and the Icinga Web 2 URL are persisted as
// locked, since the web interface then won't offer the automated source setup
// and does not need the credentials. Only the Icinga Web 2 URL is loaded from
// the database in that case, and only if it wasn't supplied. Otherwise, the
// supplied username, password and Icinga Web 2 URL are persisted as locked,
// and the URL and any missing values are loaded from unlocked rows. Unlocked
// HTTP credentials are ignored when the database URL uses a Unix socket.
//
// The merged config is validated with PasswordFile cleared, because the
// password has already been resolved at this point. config is only updated if
// validation succeeds, and its original PasswordFile is kept. Database changes
// are committed before validation and are not rolled back if it fails.
func SyncNotificationsConfig(ctx context.Context, db *database.DB, config *source.Config, clusterUuid types.UUID) error {
	tmpConfig := *config
	if tmpConfig.Url == "" {
		// config.Validate() skips loading the password file if the URL is empty, so we need to do it here.
		if err := baseConfig.LoadPasswordFile(&tmpConfig.Password, tmpConfig.PasswordFile); err != nil {
			return err
		}
	}
	// Revalidation of the config will fail if the password file is set, so we need to clear it here.
	tmpConfig.PasswordFile = ""

	var toDb []schema.Config
	if webUrl := tmpConfig.Icingaweb2Url; webUrl != "" {
		toDb = append(toDb, schema.Config{
			ClusterUuid: clusterUuid,
			Key:         schema.ConfigKeyNotificationsIcingaWeb2Url,
			Value:       webUrl,
			Locked:      types.MakeBool(true),
		})
	}

	if tmpConfig.Url != "" {
		// If the Icinga Notifications URL is locked, then Icinga for Kubernetes Web
		// won't provide the automated setup of the Icinga Notifications source,
		// so we don't need to store the credentials in the database.
		toDb = append(toDb, schema.Config{
			ClusterUuid: clusterUuid,
			Key:         schema.ConfigKeyNotificationsUrl,
			Value:       tmpConfig.Url,
			Locked:      types.MakeBool(true),
		})

		if tmpConfig.Icingaweb2Url == "" {
			if err := db.GetContext(
				ctx,
				&tmpConfig.Icingaweb2Url,
				db.Rebind(fmt.Sprintf(
					`SELECT "value" FROM %q WHERE "cluster_uuid" = ? AND "key" = ? AND "locked" = ?`,
					database.TableName(schema.Config{}),
				)),
				clusterUuid,
				schema.ConfigKeyNotificationsIcingaWeb2Url,
				types.MakeBool(false),
			); err != nil && !errors.Is(err, sql.ErrNoRows) {
				return errors.Wrap(err, "cannot select Icinga Notifications config")
			}
		}
	} else {
		if tmpConfig.Username != "" {
			toDb = append(toDb, schema.Config{
				ClusterUuid: clusterUuid,
				Key:         schema.ConfigKeyNotificationsUsername,
				Value:       tmpConfig.Username,
				Locked:      types.MakeBool(true),
			})
		}

		if tmpConfig.Password != "" {
			toDb = append(toDb, schema.Config{
				ClusterUuid: clusterUuid,
				Key:         schema.ConfigKeyNotificationsPassword,
				Value:       tmpConfig.Password,
				Locked:      types.MakeBool(true),
			})
		}

		if err := db.ExecTx(ctx, nil, func(ctx context.Context, tx *sqlx.Tx) error {
			query := db.BuildSelectStmt(&schema.Config{}, &schema.Config{}) + ` WHERE "cluster_uuid" = ? AND "key" LIKE ? AND "locked" = ?`
			rows, err := tx.QueryxContext(ctx, db.Rebind(query), clusterUuid, `notifications.%`, types.MakeBool(false))
			if err != nil {
				return errors.Wrap(err, "cannot fetch Icinga Notifications config from DB")
			}
			defer func() { _ = rows.Close() }()

			var stored []schema.Config
			for rows.Next() {
				var r schema.Config
				if err := rows.StructScan(&r); err != nil {
					return errors.Wrap(err, "cannot fetch Icinga Notifications config from DB")
				}

				stored = append(stored, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			mergeNotificationsConfig(&tmpConfig, stored)

			return nil
		}); err != nil {
			return errors.Wrap(err, "cannot retrieve Icinga Notifications config")
		}
	}

	if err := db.ExecTx(ctx, nil, func(ctx context.Context, tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(
			ctx,
			db.Rebind(fmt.Sprintf(
				`DELETE FROM %q WHERE "cluster_uuid" = ? AND "key" LIKE ? AND "locked" = ?`,
				database.TableName(&schema.Config{}),
			)),
			clusterUuid,
			`notifications.%`,
			types.MakeBool(true),
		); err != nil {
			return errors.Wrap(err, "cannot delete Icinga Notifications config")
		}

		if len(toDb) > 0 {
			stmt, _ := db.BuildUpsertStmt(schema.Config{})
			if _, err := tx.NamedExecContext(ctx, stmt, toDb); err != nil {
				return errors.Wrap(err, "cannot upsert Icinga Notifications config")
			}
		}

		return nil
	}); err != nil {
		return errors.Wrap(err, "transaction failed")
	}

	if err := tmpConfig.Validate(); err != nil {
		return errors.Wrap(err, "Icinga Notifications config is invalid")
	}

	pf := config.PasswordFile
	*config = tmpConfig
	// Reassign the password file to the config to preserve the original state.
	config.PasswordFile = pf

	return nil
}

// mergeNotificationsConfig fills missing local values from unlocked database rows.
// Unix URLs ignore stored HTTP credentials regardless of database row ordering.
// Local credentials remain intact so validation can reject incompatible locks.
func mergeNotificationsConfig(config *source.Config, stored []schema.Config) {
	for _, row := range stored {
		if row.Key == schema.ConfigKeyNotificationsUrl {
			config.Url = row.Value
		}
	}

	u, _ := url.Parse(config.Url)
	unixSocket := u != nil && u.Scheme == "unix"
	for _, row := range stored {
		switch row.Key {
		case schema.ConfigKeyNotificationsUrl, schema.ConfigKeyPrometheusUrl,
			schema.ConfigKeyPrometheusInsecure, schema.ConfigKeyPrometheusUsername, schema.ConfigKeyPrometheusPassword:
			continue
		case schema.ConfigKeyNotificationsUsername:
			if !unixSocket && config.Username == "" {
				config.Username = row.Value
			}
		case schema.ConfigKeyNotificationsPassword:
			if !unixSocket && config.Password == "" {
				config.Password = row.Value
			}
		case schema.ConfigKeyNotificationsIcingaWeb2Url:
			if config.Icingaweb2Url == "" {
				config.Icingaweb2Url = row.Value
			}
		}
	}
}
