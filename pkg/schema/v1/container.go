package v1

import (
	"database/sql"
	"github.com/icinga/icinga-kubernetes/pkg/contracts"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/types"
)

type Container struct {
	PodMeta
	Name           string
	Image          string
	CpuLimits      int64
	CpuRequests    int64
	MemoryLimits   int64
	MemoryRequests int64
	State          sql.NullString
	StateDetails   string
	Ready          types.Bool
	Started        types.Bool
	RestartCount   int32
	Devices        []*ContainerDevice `db:"-"`
	Mounts         []*ContainerMount  `db:"-"`
}

func (c *Container) Relations() []database.Relation {
	fk := database.WithForeignKey("container_id")

	return []database.Relation{
		database.HasMany(c.Devices, fk),
		database.HasMany(c.Mounts, fk),
	}
}

type ContainerMeta struct {
	contracts.Meta
	ContainerId types.Binary
}

func (cm *ContainerMeta) Fingerprint() contracts.FingerPrinter {
	return cm
}

func (cm *ContainerMeta) ParentID() types.Binary {
	return cm.ContainerId
}

type ContainerDevice struct {
	ContainerMeta
	Name string
	Path string
}

type ContainerMount struct {
	ContainerMeta
	VolumeName string
	Path       string
	SubPath    string
	ReadOnly   types.Bool
}
