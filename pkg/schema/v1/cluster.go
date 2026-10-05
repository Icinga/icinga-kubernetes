package v1

import (
	"database/sql"

	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/database"
)

type Cluster struct {
	Uuid types.UUID
	Name sql.NullString
}

func (c *Cluster) Relations() []database.Relation {
	fk := database.WithForeignKey("cluster_uuid")

	return []database.Relation{
		database.HasMany([]*Config(nil), fk),
		database.HasMany([]*ConfigMap(nil), fk),
		database.HasMany([]*CronJob(nil), fk),
		database.HasMany([]*DaemonSet(nil), fk),
		database.HasMany([]*Deployment(nil), fk),
		database.HasMany([]*EndpointSlice(nil), fk),
		database.HasMany([]*Event(nil), fk),
		database.HasMany([]*Ingress(nil), fk),
		database.HasMany([]*Job(nil), fk),
		database.HasMany([]*Instance(nil), fk),
		database.HasMany([]*Namespace(nil), fk),
		database.HasMany([]*Node(nil), fk),
		database.HasMany([]*PersistentVolume(nil), fk),
		database.HasMany([]*Pod(nil), fk),
		database.HasMany([]*PrometheusClusterMetric(nil), fk),
		database.HasMany([]*Pvc(nil), fk),
		database.HasMany([]*ReplicaSet(nil), fk),
		database.HasMany([]*Secret(nil), fk),
		database.HasMany([]*Service(nil), fk),
		database.HasMany([]*StatefulSet(nil), fk),
	}
}
