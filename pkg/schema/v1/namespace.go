package v1

import (
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/types"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

type Namespace struct {
	Meta
	Id         types.Binary
	Phase      string
	Conditions []NamespaceCondition `db:"-"`
}

type NamespaceCondition struct {
	DeploymentId   types.Binary
	Type           string
	Status         string
	LastTransition types.UnixMilli
	Reason         string
	Message        string
}

func NewNamespace() Resource {
	return &Namespace{}
}

func (n *Namespace) Obtain(k8s kmetav1.Object) {
	n.ObtainMeta(k8s)

	namespace := k8s.(*kcorev1.Namespace)

	n.Id = types.Checksum(namespace.Name)
	n.Phase = strings.ToLower(string(namespace.Status.Phase))

	for _, condition := range namespace.Status.Conditions {
		n.Conditions = append(n.Conditions, NamespaceCondition{
			DeploymentId:   n.Id,
			Type:           string(condition.Type),
			Status:         string(condition.Status),
			LastTransition: types.UnixMilli(condition.LastTransitionTime.Time),
			Reason:         condition.Reason,
			Message:        condition.Message,
		})
	}
}

func (n *Namespace) Relations() database.Relations {
	return database.Relations{
		database.HasMany[NamespaceCondition]{
			Entities:    n.Conditions,
			ForeignKey_: "namespace_id",
		},
	}
}
