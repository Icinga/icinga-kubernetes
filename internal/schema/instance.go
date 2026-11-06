package schema

import (
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/internal/kdatabase"
)

type Instance struct {
	kdatabase.EntityStub
	Uuid                   types.UUID
	ClusterUuid            types.UUID
	Version                string
	KubernetesVersion      types.String
	KubernetesHeartbeat    types.UnixMilli
	KubernetesApiReachable types.Bool
	Message                types.String
	Heartbeat              types.UnixMilli
}

func (Instance) TableName() string {
	return "kubernetes_instance"
}

func (i Instance) ID() database.ID {
	return i.Uuid
}
