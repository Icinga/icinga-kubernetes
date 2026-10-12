package v1

import (
	"database/sql"

	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/kdatabase"
)

type Instance struct {
	kdatabase.EntityStub
	Uuid                   types.UUID
	ClusterUuid            types.UUID
	Version                string
	KubernetesVersion      sql.NullString
	KubernetesHeartbeat    types.UnixMilli
	KubernetesApiReachable types.Bool
	Message                sql.NullString
	Heartbeat              types.UnixMilli
}

func (Instance) TableName() string {
	return "kubernetes_instance"
}

func (i Instance) ID() database.ID {
	return i.Uuid
}
