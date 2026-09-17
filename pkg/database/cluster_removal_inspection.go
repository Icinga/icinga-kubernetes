package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/icinga/icinga-go-library/types"
	"github.com/pkg/errors"
)

// ClusterRemovalState describes what the database currently says about a
// cluster's daemon lifecycle from the perspective of cluster removal.
type ClusterRemovalState string

const (
	ClusterRemovalStateMissing    ClusterRemovalState = "missing"
	ClusterRemovalStateNoInstance ClusterRemovalState = "no_instance"
	ClusterRemovalStateActive     ClusterRemovalState = "active"
	ClusterRemovalStateStale      ClusterRemovalState = "stale"
)

// ClusterRemovalInspection is the read-only result used by operator-facing
// callers before they invoke the destructive RemoveCluster primitive.
type ClusterRemovalInspection struct {
	ClusterUuid     types.UUID
	Name            sql.NullString
	State           ClusterRemovalState
	InstanceCount   int64
	LatestHeartbeat time.Time
}

// InspectClusterRemoval inspects the cluster and its daemon instances without
// changing database state.
//
// activeWithin is intentionally supplied by the caller. The database layer does
// not treat the Web module's UI health threshold as a deletion policy.
func (db *Database) InspectClusterRemoval(
	ctx context.Context,
	clusterUuid types.UUID,
	activeWithin time.Duration,
) (ClusterRemovalInspection, error) {
	inspection := ClusterRemovalInspection{
		ClusterUuid: clusterUuid,
	}

	if activeWithin <= 0 {
		return inspection, errors.New(
			"cluster removal active window must be greater than zero",
		)
	}

	clusterTable := db.QuoteIdentifier("cluster")
	instanceTable := db.QuoteIdentifier("kubernetes_instance")
	uuidColumn := db.QuoteIdentifier("uuid")
	nameColumn := db.QuoteIdentifier("name")
	clusterUuidColumn := db.QuoteIdentifier("cluster_uuid")
	heartbeatColumn := db.QuoteIdentifier("heartbeat")

	query := fmt.Sprintf(
		`SELECT c.%s, COUNT(ki.%s), MAX(ki.%s)
FROM %s AS c
LEFT JOIN %s AS ki ON ki.%s = c.%s
WHERE c.%s = ?
GROUP BY c.%s, c.%s`,
		nameColumn,
		uuidColumn,
		heartbeatColumn,
		clusterTable,
		instanceTable,
		clusterUuidColumn,
		uuidColumn,
		uuidColumn,
		uuidColumn,
		nameColumn,
	)

	var latestHeartbeat sql.NullInt64

	err := db.QueryRowxContext(
		ctx,
		db.Rebind(query),
		clusterUuid,
	).Scan(
		&inspection.Name,
		&inspection.InstanceCount,
		&latestHeartbeat,
	)
	if errors.Is(err, sql.ErrNoRows) {
		inspection.State = ClusterRemovalStateMissing

		return inspection, nil
	}
	if err != nil {
		return inspection, errors.Wrap(
			err,
			"cannot inspect cluster removal state",
		)
	}

	if latestHeartbeat.Valid {
		inspection.LatestHeartbeat = time.UnixMilli(
			latestHeartbeat.Int64,
		).UTC()
	}

	inspection.State = classifyClusterRemovalState(
		inspection.InstanceCount,
		inspection.LatestHeartbeat,
		time.Now(),
		activeWithin,
	)

	return inspection, nil
}

func classifyClusterRemovalState(
	instanceCount int64,
	latestHeartbeat time.Time,
	now time.Time,
	activeWithin time.Duration,
) ClusterRemovalState {
	if instanceCount == 0 {
		return ClusterRemovalStateNoInstance
	}

	cutoff := now.Add(-activeWithin)
	if !latestHeartbeat.Before(cutoff) {
		return ClusterRemovalStateActive
	}

	return ClusterRemovalStateStale
}
