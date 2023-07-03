package v1

import (
	"github.com/icinga/icinga-kubernetes/pkg/contracts"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/types"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

type Namespace struct {
	ResourceMeta
	Phase      string
	Conditions []*NamespaceCondition `json:"-" db:"-"`
}

type NamespaceMeta struct {
	contracts.Meta
	NamespaceId types.Binary
}

func (nm *NamespaceMeta) Fingerprint() contracts.FingerPrinter {
	return nm
}

func (nm *NamespaceMeta) ParentID() types.Binary {
	return nm.NamespaceId
}

type NamespaceCondition struct {
	NamespaceMeta
	Type           string
	Status         string
	LastTransition types.UnixMilli
	Reason         string
	Message        string
}

func NewNamespace() contracts.Entity {
	return &Namespace{}
}

func (n *Namespace) Obtain(k8s kmetav1.Object) {
	n.ObtainMeta(k8s)

	namespace := k8s.(*kcorev1.Namespace)

	n.Id = types.Checksum(namespace.Name)
	n.Phase = strings.ToLower(string(namespace.Status.Phase))

	n.PropertiesChecksum = types.Checksum(MustMarshalJSON(n))

	for _, condition := range namespace.Status.Conditions {
		namespaceCond := &NamespaceCondition{
			NamespaceMeta: NamespaceMeta{
				NamespaceId: n.Id,
				Meta:        contracts.Meta{Id: types.Checksum(types.MustPackSlice(n.Id, condition.Type))},
			},
			Type:           string(condition.Type),
			Status:         string(condition.Status),
			LastTransition: types.UnixMilli(condition.LastTransitionTime.Time),
			Reason:         condition.Reason,
			Message:        condition.Message,
		}
		namespaceCond.PropertiesChecksum = types.Checksum(MustMarshalJSON(namespaceCond))

		n.Conditions = append(n.Conditions, namespaceCond)
	}
}

func (n *Namespace) Relations() []database.Relation {
	fk := database.WithForeignKey("namespace_id")

	return []database.Relation{
		database.HasMany(n.Conditions, fk),
	}
}
