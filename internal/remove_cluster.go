package internal

import (
	"context"
	"fmt"

	"github.com/icinga/icinga-go-library/com"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type clusterRemovalUuid struct {
	Uuid types.UUID
}

type clusterRemovalScope struct {
	ClusterUuid types.UUID
}

func clusterRemovalContainerTypes() []any {
	return []any{
		&schemav1.Container{},
		&schemav1.InitContainer{},
		&schemav1.SidecarContainer{},
	}
}

func clusterRemovalContainerMetricRelation() database.Relation {
	return database.HasMany(
		[]*schemav1.PrometheusContainerMetric(nil),
		database.WithForeignKey("container_uuid"),
	)
}

func deleteClusterContainerRows(
	ctx context.Context,
	db *database.Database,
	clusterUuid types.UUID,
	container any,
	from any,
	features ...database.Feature,
) error {
	podTable := database.TableName(&schemav1.Pod{})
	query := db.BuildSelectStmt(container, clusterRemovalUuid{}) + fmt.Sprintf(
		" WHERE %s IN (SELECT %s FROM %s WHERE %s=:cluster_uuid)",
		db.QuoteIdentifier("pod_uuid"),
		db.QuoteIdentifier("uuid"),
		db.QuoteIdentifier(podTable),
		db.QuoteIdentifier("cluster_uuid"),
	)

	g, ctx := errgroup.WithContext(ctx)

	entities, errs := db.YieldAll(
		ctx,
		func() (any, error) { return &clusterRemovalUuid{}, nil },
		query,
		&clusterRemovalScope{ClusterUuid: clusterUuid},
	)
	com.ErrgroupReceive(g, errs)

	ids := make(chan any, db.Options.MaxPlaceholdersPerStatement)

	g.Go(func() error {
		defer close(ids)

		for {
			select {
			case entity, more := <-entities:
				if !more {
					return nil
				}

				row, ok := entity.(*clusterRemovalUuid)
				if !ok {
					return errors.Errorf("unexpected cluster removal row %T", entity)
				}

				select {
				case ids <- row.Uuid:
				case <-ctx.Done():
					return ctx.Err()
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	g.Go(func() error {
		return db.DeleteStreamed(ctx, from, ids, features...)
	})

	if err := g.Wait(); err != nil {
		return errors.Wrapf(
			err,
			"cannot remove %s rows for cluster %s",
			database.TableName(from),
			clusterUuid.String(),
		)
	}

	return nil
}

func RemoveCluster(ctx context.Context, db *database.Database, clusterUuid types.UUID) error {
	metricRelation := clusterRemovalContainerMetricRelation()

	for _, container := range clusterRemovalContainerTypes() {
		if err := deleteClusterContainerRows(
			ctx,
			db,
			clusterUuid,
			container,
			metricRelation,
			database.WithBlocking(),
		); err != nil {
			return err
		}

		if err := deleteClusterContainerRows(
			ctx,
			db,
			clusterUuid,
			container,
			container,
			database.WithBlocking(),
			database.WithCascading(),
		); err != nil {
			return err
		}
	}

	ids := make(chan any, 1)
	ids <- clusterUuid
	close(ids)

	if err := db.DeleteStreamed(
		ctx,
		&schemav1.Cluster{},
		ids,
		database.WithBlocking(),
		database.WithCascading(),
	); err != nil {
		return errors.Wrapf(err, "cannot remove cluster %s", clusterUuid.String())
	}

	return nil
}
