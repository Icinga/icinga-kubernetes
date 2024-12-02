package cluster

import (
	"context"
	"github.com/icinga/icinga-go-library/types"
)

// Private type to prevent collisions with other context keys
type contextKey string

// clusterUuidContextKey is the key for Cluster values in contexts.
var clusterUuidContextKey = contextKey("cluster_uuid")

// NewClusterUuidContext creates a new context that carries the provided cluster UUID.
// The new context is derived from the given parent context and associates the cluster UUID
// with a predefined key (clusterContextKey).
func NewClusterUuidContext(parent context.Context, clusterUuid types.UUID) context.Context {
	return context.WithValue(parent, clusterUuidContextKey, clusterUuid)
}

// ClusterUuidFromContext returns the uuid value of the cluster stored in ctx, if any:
//
//	clusterUuid, ok := ClusterUuidFromContext(ctx)
//	if !ok {
//		// Error handling.
//	}
func ClusterUuidFromContext(ctx context.Context) types.UUID {
	clusterUuid, ok := ctx.Value(clusterUuidContextKey).(types.UUID)
	if !ok {
		panic("cluster not found in context")
	}

	return clusterUuid
}
