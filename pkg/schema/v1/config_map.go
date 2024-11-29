package v1

import (
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

type ConfigMap struct {
	Meta
	Immutable            types.Bool
	Labels               []Label               `db:"-"`
	ConfigMapLabels      []ConfigMapLabel      `db:"-"`
	Annotations          []Annotation          `db:"-"`
	ConfigMapAnnotations []ConfigMapAnnotation `db:"-"`
	ResourceAnnotations  []ResourceAnnotation  `db:"-"`
}

type ConfigMapLabel struct {
	ConfigMapUuid types.UUID
	LabelUuid     types.UUID
}

type ConfigMapAnnotation struct {
	ConfigMapUuid  types.UUID
	AnnotationUuid types.UUID
}

func NewConfigMap() Resource {
	return &ConfigMap{}
}

func (c *ConfigMap) Obtain(k8s kmetav1.Object) {
	c.ObtainMeta(k8s)

	configMap := k8s.(*kcorev1.ConfigMap)

	var immutable bool
	if configMap.Immutable != nil {
		immutable = *configMap.Immutable
	}
	c.Immutable = types.Bool{
		Bool:  immutable,
		Valid: true,
	}

	for labelName, labelValue := range configMap.Labels {
		labelUuid := NewUUID(c.Uuid, strings.ToLower(labelName+":"+labelValue))
		c.Labels = append(c.Labels, Label{
			Uuid:  labelUuid,
			Name:  labelName,
			Value: labelValue,
		})
		c.ConfigMapLabels = append(c.ConfigMapLabels, ConfigMapLabel{
			ConfigMapUuid: c.Uuid,
			LabelUuid:     labelUuid,
		})
	}

	for annotationName, annotationValue := range configMap.Annotations {
		annotationUuid := NewUUID(c.Uuid, strings.ToLower(annotationName+":"+annotationValue))
		c.Annotations = append(c.Annotations, Annotation{
			Uuid:  annotationUuid,
			Name:  annotationName,
			Value: annotationValue,
		})
		c.ConfigMapAnnotations = append(c.ConfigMapAnnotations, ConfigMapAnnotation{
			ConfigMapUuid:  c.Uuid,
			AnnotationUuid: annotationUuid,
		})
		c.ResourceAnnotations = append(c.ResourceAnnotations, ResourceAnnotation{
			ResourceUuid:   c.Uuid,
			AnnotationUuid: annotationUuid,
		})
	}
}

func (c *ConfigMap) Relations() []database.Relation {
	fk := database.WithForeignKey("config_map_uuid")

	return []database.Relation{
		database.HasMany(c.Labels, database.WithoutCascadeDelete()),
		database.HasMany(c.ConfigMapLabels, fk),
		database.HasMany(c.ConfigMapAnnotations, fk),
		database.HasMany(c.Annotations, database.WithoutCascadeDelete()),
		database.HasMany(c.ResourceAnnotations, fk),
	}
}
