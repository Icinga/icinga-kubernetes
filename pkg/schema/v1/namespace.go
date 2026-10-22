package v1

import (
	"strings"

	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/kdatabase"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Namespace struct {
	Meta
	Phase               string
	Yaml                string
	Conditions          []NamespaceCondition `db:"-"`
	Labels              []Label              `db:"-"`
	NamespaceLabels     []NamespaceLabel     `db:"-"`
	ResourceLabels      []ResourceLabel      `db:"-"`
	Annotations         []Annotation         `db:"-"`
	ResourceAnnotations []ResourceAnnotation `db:"-"`
	Favorites           []Favorite           `db:"-"`
}

type NamespaceCondition struct {
	kdatabase.EntityStub
	NamespaceUuid  types.UUID
	Type           string
	Status         string
	LastTransition types.UnixMilli
	Reason         string
	Message        string
}

func (n NamespaceCondition) ID() database.ID {
	return compoundId{id: n.NamespaceUuid.String() + n.Type}
}

type NamespaceLabel struct {
	kdatabase.EntityStub
	NamespaceUuid types.UUID
	LabelUuid     types.UUID
}

func (n NamespaceLabel) ID() database.ID {
	return compoundId{id: n.NamespaceUuid.String() + n.LabelUuid.String()}
}

func NewNamespace() Resource {
	return &Namespace{}
}

func (n *Namespace) Obtain(k8s kmetav1.Object, clusterUuid types.UUID) {
	n.ObtainMeta(k8s, clusterUuid)

	namespace := k8s.(*kcorev1.Namespace)

	n.Phase = string(namespace.Status.Phase)

	for _, condition := range namespace.Status.Conditions {
		n.Conditions = append(n.Conditions, NamespaceCondition{
			NamespaceUuid:  n.Uuid,
			Type:           string(condition.Type),
			Status:         string(condition.Status),
			LastTransition: types.UnixMilli(condition.LastTransitionTime.Time),
			Reason:         condition.Reason,
			Message:        condition.Message,
		})
	}

	for labelName, labelValue := range namespace.Labels {
		labelUuid := NewUUID(n.Uuid, strings.ToLower(labelName+":"+labelValue))
		n.Labels = append(n.Labels, Label{
			Uuid:  labelUuid,
			Name:  labelName,
			Value: labelValue,
		})
		n.NamespaceLabels = append(n.NamespaceLabels, NamespaceLabel{
			NamespaceUuid: n.Uuid,
			LabelUuid:     labelUuid,
		})
		n.ResourceLabels = append(n.ResourceLabels, ResourceLabel{
			ResourceUuid: n.Uuid,
			LabelUuid:    labelUuid,
		})
	}

	for annotationName, annotationValue := range namespace.Annotations {
		annotationUuid := NewUUID(n.Uuid, strings.ToLower(annotationName+":"+annotationValue))
		n.Annotations = append(n.Annotations, Annotation{
			Uuid:         annotationUuid,
			ResourceUuid: n.Uuid,
			Name:         annotationName,
			Value:        annotationValue,
		})
		n.ResourceAnnotations = append(n.ResourceAnnotations, ResourceAnnotation{
			ResourceUuid:   n.Uuid,
			AnnotationUuid: annotationUuid,
		})
	}

	n.Yaml = MarshalYaml(namespace)
}

func (n *Namespace) Relations() []kdatabase.Relation {
	fk := kdatabase.WithForeignKey("namespace_uuid")

	return []kdatabase.Relation{
		kdatabase.HasMany(n.Conditions, fk),
		kdatabase.HasMany(n.ResourceLabels, kdatabase.WithForeignKey("resource_uuid")),
		kdatabase.HasMany(n.Labels, kdatabase.WithoutCascadeDelete()),
		kdatabase.HasMany(n.NamespaceLabels, fk),
		kdatabase.HasMany(n.ResourceAnnotations, kdatabase.WithForeignKey("resource_uuid")),
		kdatabase.HasMany(n.Annotations, kdatabase.WithForeignKey("resource_uuid")),
		kdatabase.HasMany(n.Favorites, kdatabase.WithForeignKey("resource_uuid")),
	}
}
