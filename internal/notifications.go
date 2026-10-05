package internal

import (
	"context"
	"fmt"

	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/internal/notifications"
	"github.com/icinga/icinga-kubernetes/pkg/schema"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

func SyncNotificationsConfig(ctx context.Context, db *database.DB, config *notifications.Config, clusterUuid types.UUID) error {
	_true := types.Bool{Bool: true, Valid: true}

	if config.Url != "" {
		toDb := []schema.Config{
			{ClusterUuid: clusterUuid, Key: schema.ConfigKeyNotificationsUrl, Value: config.Url, Locked: _true},
			{ClusterUuid: clusterUuid, Key: schema.ConfigKeyNotificationsUsername, Value: config.Username, Locked: _true},
			{ClusterUuid: clusterUuid, Key: schema.ConfigKeyNotificationsPassword, Value: config.Password, Locked: _true},
		}

		if err := db.ExecTx(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
			if kwebUrl := config.KubernetesWebUrl; kwebUrl != "" {
				toDb = append(toDb, schema.Config{
					ClusterUuid: clusterUuid,
					Key:         schema.ConfigKeyNotificationsKubernetesWebUrl,
					Value:       kwebUrl,
					Locked:      _true,
				})
			} else {
				if err := tx.SelectContext(ctx, &config.KubernetesWebUrl, fmt.Sprintf(
					`SELECT %q FROM %q`,
					schema.ConfigKeyNotificationsKubernetesWebUrl,
					database.TableName(schema.Config{})),
				); err != nil {
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
		if err := db.ExecTx(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
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

			rows, err := tx.QueryxContext(ctx, db.BuildSelectStmt(&schema.Config{}, &schema.Config{}))
			if err != nil {
				return errors.Wrap(err, "cannot fetch Icinga Notifications config from DB")
			}

			for rows.Next() {
				var r schema.Config
				if err := rows.StructScan(&r); err != nil {
					return errors.Wrap(err, "cannot fetch Icinga Notifications config from DB")
				}

				switch r.Key {
				case schema.ConfigKeyNotificationsUrl:
					config.Url = r.Value
				case schema.ConfigKeyNotificationsUsername:
					config.Username = r.Value
				case schema.ConfigKeyNotificationsPassword:
					config.Password = r.Value
				case schema.ConfigKeyNotificationsKubernetesWebUrl:
					config.KubernetesWebUrl = r.Value
				}
			}

			return nil
		}); err != nil {
			return errors.Wrap(err, "cannot retrieve Icinga Notifications config")
		}
	}

	return nil
}
