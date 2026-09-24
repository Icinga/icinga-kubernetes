package internal

import (
	"context"
	"fmt"

	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

type clusterRemovalUuid struct {
	Uuid types.UUID `db:"uuid"`
}

func RemoveCluster(ctx context.Context, db *database.Database, clusterUuid types.UUID) error {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "cannot begin cluster removal transaction")
	}
	defer func() {
		_ = tx.Rollback()
	}()

	for _, container := range []any{&schemav1.Container{}, &schemav1.InitContainer{}, &schemav1.SidecarContainer{}} {
		metric := database.HasMany([]schemav1.PrometheusContainerMetric{}, database.WithForeignKey("container_uuid"))
		if err := deletePodScoped(ctx, db, tx, container, metric, clusterUuid); err != nil {
			return err
		}
		if err := deletePodScoped(ctx, db, tx, container, container, clusterUuid, database.WithCascading()); err != nil {
			return err
		}
	}

	for _, cleanup := range []struct {
		parent   any
		relation database.Relation
	}{
		{&schemav1.Node{}, database.HasMany([]schemav1.PrometheusNodeMetric{}, database.WithForeignKey("node_uuid"))},
		{&schemav1.Pod{}, database.HasMany([]schemav1.PrometheusPodMetric{}, database.WithForeignKey("pod_uuid"))},
		{&schemav1.Pod{}, database.HasMany([]schemav1.ServicePod{}, database.WithForeignKey("pod_uuid"))},
	} {
		if err := deleteClusterScoped(ctx, db, tx, cleanup.parent, cleanup.relation, clusterUuid); err != nil {
			return err
		}
	}

	for _, resource := range []any{
		schemav1.NewConfigMap(),
		schemav1.NewCronJob(),
		schemav1.NewDaemonSet(),
		schemav1.NewDeployment(),
		schemav1.NewEndpointSlice(),
		schemav1.NewEvent(),
		schemav1.NewIngress(),
		schemav1.NewJob(),
		schemav1.NewNamespace(),
		schemav1.NewNode(),
		&schemav1.Pod{},
		schemav1.NewPvc(),
		schemav1.NewReplicaSet(),
		schemav1.NewSecret(),
		schemav1.NewStatefulSet(),
	} {
		if err := deleteClusterScoped(ctx, db, tx, resource, resource, clusterUuid, database.WithCascading()); err != nil {
			return err
		}
	}

	if err := deleteSpecialClusterResource(ctx, db, tx, schemav1.NewPersistentVolume(), persistentVolumeRemovalRelations(), clusterUuid); err != nil {
		return err
	}
	if err := deleteSpecialClusterResource(ctx, db, tx, &schemav1.Service{}, serviceRemovalRelations(), clusterUuid); err != nil {
		return err
	}

	for _, entity := range []any{&schemav1.PrometheusClusterMetric{}, &schemav1.Config{}, &schemav1.Instance{}} {
		if err := deleteClusterRows(ctx, db, tx, entity, "cluster_uuid", clusterUuid); err != nil {
			return err
		}
	}

	if err := deleteID(ctx, db, tx, &schemav1.Cluster{}, clusterUuid); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "cannot commit cluster removal transaction")
	}

	return nil
}

func deleteClusterScoped(
	ctx context.Context,
	db *database.Database,
	tx *sqlx.Tx,
	selectFrom any,
	deleteFrom any,
	clusterUuid types.UUID,
	features ...database.Feature,
) error {
	query := db.BuildSelectStmt(selectFrom, clusterRemovalUuid{}) + ` WHERE cluster_uuid=?`
	if err := deleteSelected(ctx, db, tx, deleteFrom, query, []any{clusterUuid}, features...); err != nil {
		return errors.Wrapf(err, "cannot remove %s rows for cluster %s", database.TableName(deleteFrom), clusterUuid)
	}

	return nil
}

func deletePodScoped(
	ctx context.Context,
	db *database.Database,
	tx *sqlx.Tx,
	selectFrom any,
	deleteFrom any,
	clusterUuid types.UUID,
	features ...database.Feature,
) error {
	query := db.BuildSelectStmt(selectFrom, clusterRemovalUuid{}) +
		` WHERE pod_uuid IN (SELECT uuid FROM pod WHERE cluster_uuid=?)`

	if err := deleteSelected(ctx, db, tx, deleteFrom, query, []any{clusterUuid}, features...); err != nil {
		return errors.Wrapf(err, "cannot remove %s rows for cluster %s", database.TableName(deleteFrom), clusterUuid)
	}

	return nil
}

func deleteSpecialClusterResource(
	ctx context.Context,
	db *database.Database,
	tx *sqlx.Tx,
	resource any,
	relations []database.Relation,
	clusterUuid types.UUID,
) error {
	for _, relation := range relations {
		if err := deleteClusterScoped(ctx, db, tx, resource, relation, clusterUuid); err != nil {
			return err
		}
	}

	return deleteClusterScoped(ctx, db, tx, resource, resource, clusterUuid)
}

func deleteSelected(
	ctx context.Context,
	db *database.Database,
	tx *sqlx.Tx,
	deleteFrom any,
	query string,
	args []any,
	features ...database.Feature,
) error {
	var rows []clusterRemovalUuid

	if err := tx.SelectContext(ctx, &rows, db.Rebind(query), args...); err != nil {
		return errors.Wrapf(err, "cannot select %s rows for deletion", database.TableName(deleteFrom))
	}

	ids := make([]any, len(rows))
	for i := range rows {
		ids[i] = rows[i].Uuid
	}

	return db.DeleteTx(ctx, tx, deleteFrom, ids, features...)
}

func deleteID(ctx context.Context, db *database.Database, tx *sqlx.Tx, from any, id any) error {
	if err := db.DeleteTx(ctx, tx, from, []any{id}); err != nil {
		return errors.Wrapf(err, "cannot remove rows from %s", database.TableName(from))
	}

	return nil
}

func deleteClusterRows(
	ctx context.Context,
	db *database.Database,
	tx *sqlx.Tx,
	entity any,
	column string,
	clusterUuid types.UUID,
) error {
	query := fmt.Sprintf(
		`DELETE FROM %s WHERE %s = ?`,
		db.QuoteIdentifier(database.TableName(entity)),
		db.QuoteIdentifier(column),
	)

	if _, err := tx.ExecContext(ctx, db.Rebind(query), clusterUuid); err != nil {
		return errors.Wrapf(err, "cannot remove rows from %s", database.TableName(entity))
	}

	return nil
}

func persistentVolumeRemovalRelations() []database.Relation {
	fk := database.WithForeignKey("persistent_volume_uuid")

	return []database.Relation{
		database.HasMany([]schemav1.PersistentVolumeClaimRef{}, fk),
		database.HasMany([]schemav1.ResourceLabel{}, database.WithForeignKey("resource_uuid")),
		database.HasMany([]schemav1.PersistentVolumeLabel{}, fk),
		database.HasMany([]schemav1.ResourceAnnotation{}, database.WithForeignKey("resource_uuid")),
		database.HasMany([]schemav1.PersistentVolumeAnnotation{}, fk),
		database.HasMany([]schemav1.Favorite{}, database.WithForeignKey("resource_uuid")),
	}
}

func serviceRemovalRelations() []database.Relation {
	fk := database.WithForeignKey("service_uuid")

	return []database.Relation{
		database.HasMany([]schemav1.ServiceCondition{}, fk),
		database.HasMany([]schemav1.ServicePort{}, fk),
		database.HasMany([]schemav1.ServiceSelector{}, fk),
		database.HasMany([]schemav1.ResourceLabel{}, database.WithForeignKey("resource_uuid")),
		database.HasMany([]schemav1.ServiceLabel{}, fk),
		database.HasMany([]schemav1.ResourceAnnotation{}, database.WithForeignKey("resource_uuid")),
		database.HasMany([]schemav1.ServiceAnnotation{}, fk),
		database.HasMany([]schemav1.ServicePod{}, fk),
		database.HasMany([]schemav1.Favorite{}, database.WithForeignKey("resource_uuid")),
	}
}
