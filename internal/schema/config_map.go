package schema

import (
	"strings"

	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/internal/kdatabase"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConfigMap struct {
	Meta
	Immutable   types.Bool
	Labels      []Label      `db:"-"`
	Annotations []Annotation `db:"-"`
}

func NewConfigMap() Resource {
	return &ConfigMap{}
}

func (c *ConfigMap) Obtain(k8s kmetav1.Object, clusterUuid types.UUID) {
	c.ObtainMeta(k8s, clusterUuid)

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
			Uuid:         labelUuid,
			ResourceUuid: c.Uuid,
			Name:         labelName,
			Value:        labelValue,
		})
	}

	for annotationName, annotationValue := range configMap.Annotations {
		annotationUuid := NewUUID(c.Uuid, strings.ToLower(annotationName+":"+annotationValue))
		c.Annotations = append(c.Annotations, Annotation{
			Uuid:         annotationUuid,
			ResourceUuid: c.Uuid,
			Name:         annotationName,
			Value:        annotationValue,
		})
	}
}

func (c *ConfigMap) Relations() []kdatabase.Relation {
	return []kdatabase.Relation{
		kdatabase.HasMany(c.Labels, kdatabase.WithForeignKey("resource_uuid")),
		kdatabase.HasMany(c.Annotations, kdatabase.WithForeignKey("resource_uuid")),
	}
}
