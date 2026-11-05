package daemon

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/notifications/source"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/internal/schema"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

func SyncNotificationsConfig(ctx context.Context, db *database.DB, config *source.Config, clusterUuid types.UUID) error {
	_true := types.Bool{Bool: true, Valid: true}
	tmpConfig := *config
	// Revalidation of the config will fail if the password file is set, so we need to clear it here.
	tmpConfig.PasswordFile = ""

	if tmpConfig.Url != "" {
		toDb := []schema.Config{
			{ClusterUuid: clusterUuid, Key: schema.ConfigKeyNotificationsUrl, Value: tmpConfig.Url, Locked: _true},
			{ClusterUuid: clusterUuid, Key: schema.ConfigKeyNotificationsUsername, Value: tmpConfig.Username, Locked: _true},
			{ClusterUuid: clusterUuid, Key: schema.ConfigKeyNotificationsPassword, Value: tmpConfig.Password, Locked: _true},
		}

		if err := db.ExecTx(ctx, nil, func(ctx context.Context, tx *sqlx.Tx) error {
			if webUrl := tmpConfig.Icingaweb2Url; webUrl != "" {
				toDb = append(toDb, schema.Config{
					ClusterUuid: clusterUuid,
					Key:         schema.ConfigKeyNotificationsIcingaWeb2Url,
					Value:       webUrl,
					Locked:      _true,
				})
			} else {
				err := tx.GetContext(ctx, &tmpConfig.Icingaweb2Url, db.Rebind(fmt.Sprintf(
					`SELECT "value" FROM %q WHERE "cluster_uuid" = ? AND "key" = ? AND "locked" = ?`,
					database.TableName(schema.Config{}),
				)), clusterUuid, schema.ConfigKeyNotificationsIcingaWeb2Url, types.MakeBool(false))
				if err != nil && !errors.Is(err, sql.ErrNoRows) {
					return errors.Wrap(err, "cannot select Icinga Notifications config")
				}
			}

			if _, err := tx.ExecContext(
				ctx,
				fmt.Sprintf(
					`DELETE FROM %q WHERE "cluster_uuid" = ? AND "key" LIKE ? AND "locked" = ?`,
					database.TableName(&schema.Config{}),
				),
				clusterUuid,
				`notifications.%`,
				_true,
			); err != nil {
				return errors.Wrap(err, "cannot delete Icinga Notifications config")
			}

			stmt, _ := db.BuildUpsertStmt(schema.Config{})
			if _, err := tx.NamedExecContext(ctx, stmt, toDb); err != nil {
				return errors.Wrap(err, "cannot upsert Icinga Notifications config")
			}

			return nil
		}); err != nil {
			return errors.Wrap(err, "transaction failed")
		}
	} else {
		if err := db.ExecTx(ctx, nil, func(ctx context.Context, tx *sqlx.Tx) error {
			if _, err := tx.ExecContext(
				ctx,
				fmt.Sprintf(
					`DELETE FROM %q WHERE "cluster_uuid" = ? AND "key" LIKE ? AND "locked" = ?`,
					database.TableName(&schema.Config{}),
				),
				clusterUuid,
				`notifications.%`,
				_true,
			); err != nil {
				return errors.Wrap(err, "cannot delete Icinga Notifications config")
			}

			query := db.BuildSelectStmt(&schema.Config{}, &schema.Config{}) + ` WHERE "cluster_uuid" = ? AND "key" LIKE ?`
			rows, err := tx.QueryxContext(ctx, db.Rebind(query), clusterUuid, `notifications.%`)
			if err != nil {
				return errors.Wrap(err, "cannot fetch Icinga Notifications config from DB")
			}
			defer func() { _ = rows.Close() }()

			for rows.Next() {
				var r schema.Config
				if err := rows.StructScan(&r); err != nil {
					return errors.Wrap(err, "cannot fetch Icinga Notifications config from DB")
				}

				switch r.Key {
				case schema.ConfigKeyNotificationsUrl:
					tmpConfig.Url = r.Value
				case schema.ConfigKeyNotificationsUsername:
					tmpConfig.Username = r.Value
				case schema.ConfigKeyNotificationsPassword:
					tmpConfig.Password = r.Value
				case schema.ConfigKeyNotificationsIcingaWeb2Url:
					tmpConfig.Icingaweb2Url = r.Value
				}
			}

			return rows.Err()
		}); err != nil {
			return errors.Wrap(err, "cannot retrieve Icinga Notifications config")
		}
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
