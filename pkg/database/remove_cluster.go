package database

import (
	"context"
	"fmt"

	"github.com/icinga/icinga-go-library/types"
	v1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

type clusterRemovalRelation struct {
	table      string
	foreignKey string
}

type clusterRemovalResource struct {
	table     string
	relations []clusterRemovalRelation
}

var clusterRemovalResources = []clusterRemovalResource{
	{
		table: "config_map",
		relations: []clusterRemovalRelation{
			{table: "config_map_annotation", foreignKey: "config_map_uuid"},
			{table: "config_map_label", foreignKey: "config_map_uuid"},
			{table: "resource_annotation", foreignKey: "resource_uuid"},
			{table: "resource_label", foreignKey: "resource_uuid"},
		},
	},
	{
		table: "cron_job",
		relations: []clusterRemovalRelation{
			{table: "cron_job_annotation", foreignKey: "cron_job_uuid"},
			{table: "cron_job_label", foreignKey: "cron_job_uuid"},
			{table: "favorite", foreignKey: "resource_uuid"},
			{table: "resource_annotation", foreignKey: "resource_uuid"},
			{table: "resource_label", foreignKey: "resource_uuid"},
		},
	},
	{
		table: "daemon_set",
		relations: []clusterRemovalRelation{
			{table: "daemon_set_annotation", foreignKey: "daemon_set_uuid"},
			{table: "daemon_set_condition", foreignKey: "daemon_set_uuid"},
			{table: "daemon_set_label", foreignKey: "daemon_set_uuid"},
			{table: "daemon_set_owner", foreignKey: "daemon_set_uuid"},
			{table: "favorite", foreignKey: "resource_uuid"},
			{table: "resource_annotation", foreignKey: "resource_uuid"},
			{table: "resource_label", foreignKey: "resource_uuid"},
		},
	},
	{
		table: "deployment",
		relations: []clusterRemovalRelation{
			{table: "deployment_annotation", foreignKey: "deployment_uuid"},
			{table: "deployment_condition", foreignKey: "deployment_uuid"},
			{table: "deployment_label", foreignKey: "deployment_uuid"},
			{table: "deployment_owner", foreignKey: "deployment_uuid"},
			{table: "favorite", foreignKey: "resource_uuid"},
			{table: "resource_annotation", foreignKey: "resource_uuid"},
			{table: "resource_label", foreignKey: "resource_uuid"},
		},
	},
	{
		table: "endpoint_slice",
		relations: []clusterRemovalRelation{
			{table: "endpoint", foreignKey: "endpoint_slice_uuid"},
			{table: "endpoint_slice_label", foreignKey: "endpoint_slice_uuid"},
			{table: "endpoint_target_ref", foreignKey: "endpoint_slice_uuid"},
		},
	},
	{table: "event"},
	{
		table: "ingress",
		relations: []clusterRemovalRelation{
			{table: "favorite", foreignKey: "resource_uuid"},
			{table: "ingress_annotation", foreignKey: "ingress_uuid"},
			{table: "ingress_backend_resource", foreignKey: "ingress_uuid"},
			{table: "ingress_backend_service", foreignKey: "ingress_uuid"},
			{table: "ingress_label", foreignKey: "ingress_uuid"},
			{table: "ingress_rule", foreignKey: "ingress_uuid"},
			{table: "ingress_tls", foreignKey: "ingress_uuid"},
			{table: "resource_annotation", foreignKey: "resource_uuid"},
			{table: "resource_label", foreignKey: "resource_uuid"},
		},
	},
	{
		table: "job",
		relations: []clusterRemovalRelation{
			{table: "favorite", foreignKey: "resource_uuid"},
			{table: "job_annotation", foreignKey: "job_uuid"},
			{table: "job_condition", foreignKey: "job_uuid"},
			{table: "job_label", foreignKey: "job_uuid"},
			{table: "job_owner", foreignKey: "job_uuid"},
			{table: "resource_annotation", foreignKey: "resource_uuid"},
			{table: "resource_label", foreignKey: "resource_uuid"},
		},
	},
	{
		table: "namespace",
		relations: []clusterRemovalRelation{
			{table: "favorite", foreignKey: "resource_uuid"},
			{table: "namespace_annotation", foreignKey: "namespace_uuid"},
			{table: "namespace_condition", foreignKey: "namespace_uuid"},
			{table: "namespace_label", foreignKey: "namespace_uuid"},
			{table: "resource_annotation", foreignKey: "resource_uuid"},
			{table: "resource_label", foreignKey: "resource_uuid"},
		},
	},
	{
		table: "node",
		relations: []clusterRemovalRelation{
			{table: "favorite", foreignKey: "resource_uuid"},
			{table: "node_annotation", foreignKey: "node_uuid"},
			{table: "node_condition", foreignKey: "node_uuid"},
			{table: "node_label", foreignKey: "node_uuid"},
			{table: "node_volume", foreignKey: "node_uuid"},
			{table: "prometheus_node_metric", foreignKey: "node_uuid"},
			{table: "resource_annotation", foreignKey: "resource_uuid"},
			{table: "resource_label", foreignKey: "resource_uuid"},
		},
	},
	{
		table: "persistent_volume",
		relations: []clusterRemovalRelation{
			{table: "favorite", foreignKey: "resource_uuid"},
			{table: "persistent_volume_annotation", foreignKey: "persistent_volume_uuid"},
			{table: "persistent_volume_claim_ref", foreignKey: "persistent_volume_uuid"},
			{table: "persistent_volume_label", foreignKey: "persistent_volume_uuid"},
			{table: "resource_annotation", foreignKey: "resource_uuid"},
			{table: "resource_label", foreignKey: "resource_uuid"},
		},
	},
	{
		table: "pod",
		relations: []clusterRemovalRelation{
			{table: "favorite", foreignKey: "resource_uuid"},
			{table: "pod_annotation", foreignKey: "pod_uuid"},
			{table: "pod_condition", foreignKey: "pod_uuid"},
			{table: "pod_label", foreignKey: "pod_uuid"},
			{table: "pod_owner", foreignKey: "pod_uuid"},
			{table: "pod_pvc", foreignKey: "pod_uuid"},
			{table: "pod_volume", foreignKey: "pod_uuid"},
			{table: "prometheus_pod_metric", foreignKey: "pod_uuid"},
			{table: "resource_annotation", foreignKey: "resource_uuid"},
			{table: "resource_label", foreignKey: "resource_uuid"},
			{table: "service_pod", foreignKey: "pod_uuid"},
		},
	},
	{
		table: "pvc",
		relations: []clusterRemovalRelation{
			{table: "favorite", foreignKey: "resource_uuid"},
			{table: "pvc_annotation", foreignKey: "pvc_uuid"},
			{table: "pvc_condition", foreignKey: "pvc_uuid"},
			{table: "pvc_label", foreignKey: "pvc_uuid"},
			{table: "resource_annotation", foreignKey: "resource_uuid"},
			{table: "resource_label", foreignKey: "resource_uuid"},
		},
	},
	{
		table: "replica_set",
		relations: []clusterRemovalRelation{
			{table: "favorite", foreignKey: "resource_uuid"},
			{table: "replica_set_annotation", foreignKey: "replica_set_uuid"},
			{table: "replica_set_condition", foreignKey: "replica_set_uuid"},
			{table: "replica_set_label", foreignKey: "replica_set_uuid"},
			{table: "replica_set_owner", foreignKey: "replica_set_uuid"},
			{table: "resource_annotation", foreignKey: "resource_uuid"},
			{table: "resource_label", foreignKey: "resource_uuid"},
		},
	},
	{
		table: "secret",
		relations: []clusterRemovalRelation{
			{table: "resource_annotation", foreignKey: "resource_uuid"},
			{table: "resource_label", foreignKey: "resource_uuid"},
			{table: "secret_annotation", foreignKey: "secret_uuid"},
			{table: "secret_label", foreignKey: "secret_uuid"},
		},
	},
	{
		table: "service",
		relations: []clusterRemovalRelation{
			{table: "favorite", foreignKey: "resource_uuid"},
			{table: "resource_annotation", foreignKey: "resource_uuid"},
			{table: "resource_label", foreignKey: "resource_uuid"},
			{table: "service_annotation", foreignKey: "service_uuid"},
			{table: "service_condition", foreignKey: "service_uuid"},
			{table: "service_label", foreignKey: "service_uuid"},
			{table: "service_pod", foreignKey: "service_uuid"},
			{table: "service_port", foreignKey: "service_uuid"},
			{table: "service_selector", foreignKey: "service_uuid"},
		},
	},
	{
		table: "stateful_set",
		relations: []clusterRemovalRelation{
			{table: "favorite", foreignKey: "resource_uuid"},
			{table: "resource_annotation", foreignKey: "resource_uuid"},
			{table: "resource_label", foreignKey: "resource_uuid"},
			{table: "stateful_set_annotation", foreignKey: "stateful_set_uuid"},
			{table: "stateful_set_condition", foreignKey: "stateful_set_uuid"},
			{table: "stateful_set_label", foreignKey: "stateful_set_uuid"},
			{table: "stateful_set_owner", foreignKey: "stateful_set_uuid"},
		},
	},
}

var clusterRemovalDirectTables = []string{
	"prometheus_cluster_metric",
	"config",
	"kubernetes_instance",
}

// RemoveCluster removes database state belonging to clusterUuid in one transaction.
//
// This method intentionally does not decide whether the corresponding Icinga for
// Kubernetes daemon is still active. Operator-facing callers must establish that
// lifecycle precondition before invoking RemoveCluster; otherwise an active daemon
// could recreate the cluster after this transaction commits.
//
// The annotation, label and selector lookup tables are intentionally left alone to
// match the existing resource deletion semantics, where those relations are marked
// WithoutCascadeDelete. The legacy pod_metrics table cannot be safely scoped to one
// cluster because it has no cluster UUID, and kubernetes_schema is global state.
func (db *Database) RemoveCluster(
	ctx context.Context,
	clusterUuid types.UUID,
) error {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "cannot begin cluster removal transaction")
	}

	defer func() { _ = tx.Rollback() }()

	// Loop: SELECT all resources that have cluster_uuid
	factory := v1.NewDaemonSet

	meta := &v1.Meta{ClusterUuid: clusterUuid}
	query := db.BuildSelectStmt(factory(), meta) + ` WHERE cluster_uuid=:cluster_uuid`

	deletes := make(chan any)

	entities, errs := db.YieldAll(ctx, func() (any, error) {
		return factory(), nil
	}, query, meta)
	for {
		select {
		case entity, ok := <-entities:
			if !ok {
				return ctx.Err()
			}

			select {
			case deletes <- entity.(v1.Meta).Uuid:
			case <-ctx.Done():
				return ctx.Err()
			}
		case <-ctx.Done():
			return nil
		case err := <-errs:
			return fmt.Errorf("cannot remove cluster: %w", err)
		}
	}

	if err := db.DeleteStreamed(ctx, factory(), deletes); err != nil {
		return fmt.Errorf("cannot remove cluster %s: %w", clusterUuid, err)
	}

	if err := db.removeClusterPodContainers(
		ctx,
		tx,
		clusterUuid,
	); err != nil {
		return err
	}

	for _, resource := range clusterRemovalResources {
		if err := db.removeClusterResource(
			ctx,
			tx,
			resource,
			clusterUuid,
		); err != nil {
			return err
		}
	}

	for _, table := range clusterRemovalDirectTables {
		if err := db.deleteClusterRows(
			ctx,
			tx,
			table,
			"cluster_uuid",
			clusterUuid,
		); err != nil {
			return err
		}
	}

	if err := db.deleteClusterRows(
		ctx,
		tx,
		"cluster",
		"uuid",
		clusterUuid,
	); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(
			err,
			"cannot commit cluster removal transaction",
		)
	}

	return nil
}

func (db *Database) removeClusterResource(
	ctx context.Context,
	tx *sqlx.Tx,
	resource clusterRemovalResource,
	clusterUuid types.UUID,
) error {
	for _, relation := range resource.relations {
		if err := db.deleteClusterResourceRelation(
			ctx,
			tx,
			relation,
			resource.table,
			clusterUuid,
		); err != nil {
			return err
		}
	}

	return db.deleteClusterRows(
		ctx,
		tx,
		resource.table,
		"cluster_uuid",
		clusterUuid,
	)
}

func (db *Database) deleteClusterResourceRelation(
	ctx context.Context,
	tx *sqlx.Tx,
	relation clusterRemovalRelation,
	resourceTable string,
	clusterUuid types.UUID,
) error {
	query := fmt.Sprintf(
		`DELETE FROM %s WHERE %s IN (`+
			`SELECT %s FROM %s WHERE %s = ?)`,

		db.QuoteIdentifier(relation.table),
		db.QuoteIdentifier(relation.foreignKey),
		db.QuoteIdentifier("uuid"),
		db.QuoteIdentifier(resourceTable),
		db.QuoteIdentifier("cluster_uuid"),
	)

	if _, err := tx.ExecContext(
		ctx,
		db.Rebind(query),
		clusterUuid,
	); err != nil {
		return errors.Wrapf(
			err,
			"cannot remove %s rows for cluster resource %s",
			relation.table,
			resourceTable,
		)
	}

	return nil
}

func (db *Database) removeClusterPodContainers(
	ctx context.Context,
	tx *sqlx.Tx,
	clusterUuid types.UUID,
) error {
	for _, containerTable := range []string{
		"container",
		"init_container",
		"sidecar_container",
	} {
		query := fmt.Sprintf(
			`DELETE FROM %s WHERE %s IN (`+
				`SELECT %s FROM %s WHERE %s IN (`+
				`SELECT %s FROM %s WHERE %s = ?))`,

			db.QuoteIdentifier(
				"prometheus_container_metric",
			),
			db.QuoteIdentifier("container_uuid"),
			db.QuoteIdentifier("uuid"),
			db.QuoteIdentifier(containerTable),
			db.QuoteIdentifier("pod_uuid"),
			db.QuoteIdentifier("uuid"),
			db.QuoteIdentifier("pod"),
			db.QuoteIdentifier("cluster_uuid"),
		)

		if _, err := tx.ExecContext(
			ctx,
			db.Rebind(query),
			clusterUuid,
		); err != nil {
			return errors.Wrapf(
				err,
				"cannot remove prometheus container metrics for %s",
				containerTable,
			)
		}
	}

	for _, table := range []string{
		"container_device",
		"container_log",
		"container_mount",
	} {
		relation := clusterRemovalRelation{
			table:      table,
			foreignKey: "pod_uuid",
		}

		if err := db.deleteClusterResourceRelation(
			ctx,
			tx,
			relation,
			"pod",
			clusterUuid,
		); err != nil {
			return err
		}
	}

	for _, table := range []string{
		"container",
		"init_container",
		"sidecar_container",
	} {
		relation := clusterRemovalRelation{
			table:      table,
			foreignKey: "pod_uuid",
		}

		if err := db.deleteClusterResourceRelation(
			ctx,
			tx,
			relation,
			"pod",
			clusterUuid,
		); err != nil {
			return err
		}
	}

	return nil
}

func (db *Database) deleteClusterRows(
	ctx context.Context,
	tx *sqlx.Tx,
	table string,
	column string,
	clusterUuid types.UUID,
) error {
	query := fmt.Sprintf(
		`DELETE FROM %s WHERE %s = ?`,
		db.QuoteIdentifier(table),
		db.QuoteIdentifier(column),
	)

	if _, err := tx.ExecContext(
		ctx,
		db.Rebind(query),
		clusterUuid,
	); err != nil {
		return errors.Wrapf(
			err,
			"cannot remove rows from %s",
			table,
		)
	}

	return nil
}
