package v1

import (
	"github.com/icinga/icinga-kubernetes/pkg/contracts"
	"github.com/icinga/icinga-kubernetes/pkg/types"
	"strings"
)

type Label struct {
	contracts.Meta
	PodId         types.Binary
	ReplicaSetId  types.Binary
	DeploymentId  types.Binary
	DaemonSetId   types.Binary
	StatefulSetId types.Binary
	PvcId         types.Binary
	Name          string
	Value         string
}

func NewLabel(name string, value string) *Label {
	return &Label{
		Meta:  contracts.Meta{Id: types.Checksum(strings.ToLower(name + ":" + value))},
		Name:  name,
		Value: value,
	}
}
