package v1

import (
	"database/sql"

	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/kdatabase"
)

type Cluster struct {
	kdatabase.EntityStub
	Uuid types.UUID
	Name sql.NullString
}

func (c Cluster) ID() database.ID {
	return c.Uuid
}

func (c *Cluster) Relations() []kdatabase.Relation {
	fk := kdatabase.WithForeignKey("cluster_uuid")

	// Entities that aren't comparable, e.g. because of slice fields, are given as pointers.
	return []kdatabase.Relation{
		kdatabase.HasMany([]Config(nil), fk),
		kdatabase.HasMany([]*ConfigMap(nil), fk),
		kdatabase.HasMany([]*CronJob(nil), fk),
		kdatabase.HasMany([]*DaemonSet(nil), fk),
		kdatabase.HasMany([]*Deployment(nil), fk),
		kdatabase.HasMany([]*EndpointSlice(nil), fk),
		kdatabase.HasMany([]Event(nil), fk),
		kdatabase.HasMany([]*Ingress(nil), fk),
		kdatabase.HasMany([]Instance(nil), fk),
		kdatabase.HasMany([]*Job(nil), fk),
		kdatabase.HasMany([]*Namespace(nil), fk),
		kdatabase.HasMany([]*Node(nil), fk),
		kdatabase.HasMany([]*PersistentVolume(nil), fk),
		kdatabase.HasMany([]*Pod(nil), fk),
		kdatabase.HasMany([]PrometheusClusterMetric(nil), fk),
		kdatabase.HasMany([]*Pvc(nil), fk),
		kdatabase.HasMany([]*ReplicaSet(nil), fk),
		kdatabase.HasMany([]*Secret(nil), fk),
		kdatabase.HasMany([]*Service(nil), fk),
		kdatabase.HasMany([]*StatefulSet(nil), fk),
	}
}
