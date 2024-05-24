package v1

import (
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-go-library/utils"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

type Namespace struct {
	Meta
	Id              types.Binary
	Phase           string
	Conditions      []NamespaceCondition `db:"-"`
	Labels          []Label              `db:"-"`
	NamespaceLabels []NamespaceLabel     `db:"-"`
}

type NamespaceCondition struct {
	NamespaceId    types.Binary
	Type           string
	Status         string
	LastTransition types.UnixMilli
	Reason         string
	Message        string
}

type NamespaceLabel struct {
	NamespaceId types.Binary
	LabelId     types.Binary
}

func NewNamespace() Resource {
	return &Namespace{}
}

func (n *Namespace) Obtain(k8s kmetav1.Object) {
	n.ObtainMeta(k8s)

	namespace := k8s.(*kcorev1.Namespace)

	n.Id = utils.Checksum(namespace.Name)
	n.Phase = strings.ToLower(string(namespace.Status.Phase))

	for _, condition := range namespace.Status.Conditions {
		n.Conditions = append(n.Conditions, NamespaceCondition{
			NamespaceId:    n.Id,
			Type:           string(condition.Type),
			Status:         string(condition.Status),
			LastTransition: types.UnixMilli(condition.LastTransitionTime.Time),
			Reason:         condition.Reason,
			Message:        condition.Message,
		})
	}

	for labelName, labelValue := range namespace.Labels {
		labelId := utils.Checksum(strings.ToLower(labelName + ":" + labelValue))
		n.Labels = append(n.Labels, Label{
			Id:    labelId,
			Name:  labelName,
			Value: labelValue,
		})
		n.NamespaceLabels = append(n.NamespaceLabels, NamespaceLabel{
			NamespaceId: n.Id,
			LabelId:     labelId,
		})
	}
}

func (n *Namespace) Relations() []database.Relation {
	fk := database.WithForeignKey("namespace_id")

	return []database.Relation{
		database.HasMany(n.Conditions, fk),
		database.HasMany(n.NamespaceLabels, fk),
		database.HasMany(n.Labels, database.WithoutCascadeDelete()),
	}
}
