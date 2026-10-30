package schema

import (
	"strings"

	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/internal/kdatabase"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Secret struct {
	Meta
	Type        string
	Immutable   types.Bool
	Labels      []Label      `db:"-"`
	Annotations []Annotation `db:"-"`
}

func NewSecret() Resource {
	return &Secret{}
}

func (s *Secret) Obtain(k8s kmetav1.Object, clusterUuid types.UUID) {
	s.ObtainMeta(k8s, clusterUuid)

	secret := k8s.(*kcorev1.Secret)

	s.Type = string(secret.Type)

	var immutable bool
	if secret.Immutable != nil {
		immutable = *secret.Immutable
	}
	s.Immutable = types.Bool{
		Bool:  immutable,
		Valid: true,
	}

	for labelName, labelValue := range secret.Labels {
		labelUuid := NewUUID(s.Uuid, strings.ToLower(labelName+":"+labelValue))
		s.Labels = append(s.Labels, Label{
			Uuid:         labelUuid,
			ResourceUuid: s.Uuid,
			Name:         labelName,
			Value:        labelValue,
		})
	}

	for annotationName, annotationValue := range secret.Annotations {
		annotationUuid := NewUUID(s.Uuid, strings.ToLower(annotationName+":"+annotationValue))
		s.Annotations = append(s.Annotations, Annotation{
			Uuid:         annotationUuid,
			ResourceUuid: s.Uuid,
			Name:         annotationName,
			Value:        annotationValue,
		})
	}
}

func (s *Secret) Relations() []kdatabase.Relation {
	return []kdatabase.Relation{
		kdatabase.HasMany(s.Labels, kdatabase.WithForeignKey("resource_uuid")),
		kdatabase.HasMany(s.Annotations, kdatabase.WithForeignKey("resource_uuid")),
	}
}
