package v1

import (
	"database/sql"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/types"
)

type ContainerMeta struct {
	Id    types.Binary `db:"id"`
	PodId types.Binary `db:"pod_id"`
}

type Container struct {
	ContainerMeta
	Name             string
	Image            string
	CpuLimits        int64
	CpuRequests      int64
	MemoryLimits     int64
	MemoryRequests   int64
	State            sql.NullString
	StateDetails     string
	Ready            types.Bool
	Started          types.Bool
	RestartCount     int32
	ContainerDevices []ContainerDevice `db:"-"`
	ContainerMounts  []ContainerMount  `db:"-"`
}

func (c *Container) Relations() []database.Relation {
	fk := database.WithForeignKey("container_id")

	return []database.Relation{
		database.HasMany(c.ContainerDevices, fk),
		database.HasMany(c.ContainerMounts, fk),
	}
}

type ContainerDevice struct {
	ContainerId types.Binary
	PodId       types.Binary
	Name        string
	Path        string
}

type ContainerMount struct {
	ContainerId types.Binary
	PodId       types.Binary
	VolumeName  string
	Path        string
	SubPath     sql.NullString
	ReadOnly    types.Bool
}

// Assert that Container satisfies the interface compliance.
var (
	_ database.HasRelations = (*Container)(nil)
)
