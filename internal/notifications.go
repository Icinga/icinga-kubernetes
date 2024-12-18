package internal

import (
	"context"
	"fmt"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/notifications"
	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

func SyncNotificationsConfig(ctx context.Context, db *database.DB, config *notifications.Config, clusterUuid types.UUID) error {
	_true := types.Bool{Bool: true, Valid: true}

	if config.Url != "" {
		toDb := []schemav1.Config{
			{ClusterUuid: clusterUuid, Key: schemav1.ConfigKeyNotificationsUrl, Value: config.Url, Locked: _true},
			{ClusterUuid: clusterUuid, Key: schemav1.ConfigKeyNotificationsUsername, Value: config.Username, Locked: _true},
			{ClusterUuid: clusterUuid, Key: schemav1.ConfigKeyNotificationsPassword, Value: config.Password, Locked: _true},
		}

		err := db.ExecTx(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
			if kwebUrl := config.KubernetesWebUrl; kwebUrl != "" {
				toDb = append(toDb, schemav1.Config{
					ClusterUuid: clusterUuid,
					Key:         schemav1.ConfigKeyNotificationsKubernetesWebUrl,
					Value:       kwebUrl,
					Locked:      _true,
				})
			} else {
				if err := tx.SelectContext(ctx, &config.KubernetesWebUrl, fmt.Sprintf(
					`SELECT "%s" FROM "%s"`,
					schemav1.ConfigKeyNotificationsKubernetesWebUrl,
					database.TableName(schemav1.Config{})),
				); err != nil {
					return errors.Wrap(err, "cannot select Icinga Notifications config")
				}
			}

			if _, err := tx.ExecContext(
				ctx,
				fmt.Sprintf(
					`DELETE FROM "%s" WHERE "cluster_uuid" = ? AND "key" LIKE ? AND "locked" = ?`,
					database.TableName(&schemav1.Config{}),
				),
				clusterUuid,
				`notifications.%`,
				_true,
			); err != nil {
				return errors.Wrap(err, "cannot delete Icinga Notifications config")
			}

			stmt, _ := db.BuildInsertStmt(schemav1.Config{})
			if _, err := tx.NamedExecContext(ctx, stmt, toDb); err != nil {
				return errors.Wrap(err, "cannot insert Icinga Notifications config")
			}

			return nil
		})
		if err != nil {
			return errors.Wrap(err, "cannot upsert Icinga Notifications config")
		}
	} else {
		err := db.ExecTx(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
			if _, err := tx.ExecContext(
				ctx,
				fmt.Sprintf(
					`DELETE FROM "%s" WHERE "cluster_uuid" = ? AND "key" LIKE ? AND "locked" = ?`,
					database.TableName(&schemav1.Config{}),
				),
				clusterUuid,
				`notifications.%`,
				_true,
			); err != nil {
				return errors.Wrap(err, "cannot delete Icinga Notifications config")
			}

			rows, err := tx.QueryxContext(ctx, db.BuildSelectStmt(&schemav1.Config{}, &schemav1.Config{}))
			if err != nil {
				return errors.Wrap(err, "cannot fetch Icinga Notifications config from DB")
			}

			for rows.Next() {
				var r schemav1.Config
				if err := rows.StructScan(&r); err != nil {
					return errors.Wrap(err, "cannot fetch Icinga Notifications config from DB")
				}

				switch r.Key {
				case schemav1.ConfigKeyNotificationsUrl:
					config.Url = r.Value
				case schemav1.ConfigKeyNotificationsUsername:
					config.Username = r.Value
				case schemav1.ConfigKeyNotificationsPassword:
					config.Password = r.Value
				case schemav1.ConfigKeyNotificationsKubernetesWebUrl:
					config.KubernetesWebUrl = r.Value
				}
			}

			return nil
		})
		if err != nil {
			return errors.Wrap(err, "cannot retrieve Icinga Notifications config")
		}
	}

	return nil
}
