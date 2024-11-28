package v1

import (
	"database/sql"
	"github.com/icinga/icinga-go-library/types"
)

type Instance struct {
	Uuid                   types.Binary
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
