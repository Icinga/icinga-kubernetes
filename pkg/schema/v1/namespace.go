package v1

import (
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	kserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"strings"
)

type Namespace struct {
	Meta
	Phase                string
	Yaml                 string
	Conditions           []NamespaceCondition  `db:"-"`
	Labels               []Label               `db:"-"`
	NamespaceLabels      []NamespaceLabel      `db:"-"`
	ResourceLabels       []ResourceLabel       `db:"-"`
	Annotations          []Annotation          `db:"-"`
	NamespaceAnnotations []NamespaceAnnotation `db:"-"`
	ResourceAnnotations  []ResourceAnnotation  `db:"-"`
	Favorites            []Favorite            `db:"-"`
}

type NamespaceCondition struct {
	NamespaceUuid  types.UUID
	Type           string
	Status         string
	LastTransition types.UnixMilli
	Reason         string
	Message        string
}

type NamespaceLabel struct {
	NamespaceUuid types.UUID
	LabelUuid     types.UUID
}

type NamespaceAnnotation struct {
	NamespaceUuid  types.UUID
	AnnotationUuid types.UUID
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
			Uuid:  annotationUuid,
			Name:  annotationName,
			Value: annotationValue,
		})
		n.NamespaceAnnotations = append(n.NamespaceAnnotations, NamespaceAnnotation{
			NamespaceUuid:  n.Uuid,
			AnnotationUuid: annotationUuid,
		})
		n.ResourceAnnotations = append(n.ResourceAnnotations, ResourceAnnotation{
			ResourceUuid:   n.Uuid,
			AnnotationUuid: annotationUuid,
		})
	}

	scheme := kruntime.NewScheme()
	_ = kcorev1.AddToScheme(scheme)
	codec := kserializer.NewCodecFactory(scheme).EncoderForVersion(kjson.NewYAMLSerializer(kjson.DefaultMetaFactory, scheme, scheme), kcorev1.SchemeGroupVersion)
	output, _ := kruntime.Encode(codec, namespace)
	n.Yaml = string(output)
}

func (n *Namespace) Relations() []database.Relation {
	fk := database.WithForeignKey("namespace_uuid")

	return []database.Relation{
		database.HasMany(n.Conditions, fk),
		database.HasMany(n.ResourceLabels, database.WithForeignKey("resource_uuid")),
		database.HasMany(n.Labels, database.WithoutCascadeDelete()),
		database.HasMany(n.NamespaceLabels, fk),
		database.HasMany(n.ResourceAnnotations, database.WithForeignKey("resource_uuid")),
		database.HasMany(n.Annotations, database.WithoutCascadeDelete()),
		database.HasMany(n.NamespaceAnnotations, fk),
		database.HasMany(n.Favorites, database.WithForeignKey("resource_uuid")),
	}
}
