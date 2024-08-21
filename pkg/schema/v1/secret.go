package v1

import (
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

type Secret struct {
	Meta
	Type              string
	Immutable         types.Bool
	Labels            []Label            `db:"-"`
	SecretLabels      []SecretLabel      `db:"-"`
	Annotations       []Annotation       `db:"-"`
	SecretAnnotations []SecretAnnotation `db:"-"`
}

type SecretLabel struct {
	SecretUuid types.UUID
	LabelUuid  types.UUID
}

type SecretAnnotation struct {
	SecretUuid     types.UUID
	AnnotationUuid types.UUID
}

func NewSecret() Resource {
	return &Secret{}
}

func (s *Secret) Obtain(k8s kmetav1.Object) {
	s.ObtainMeta(k8s)

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
			Uuid:  labelUuid,
			Name:  labelName,
			Value: labelValue,
		})
		s.SecretLabels = append(s.SecretLabels, SecretLabel{
			SecretUuid: s.Uuid,
			LabelUuid:  labelUuid,
		})
	}

	for annotationName, annotationValue := range secret.Annotations {
		annotationUuid := NewUUID(s.Uuid, strings.ToLower(annotationName+":"+annotationValue))
		s.Annotations = append(s.Annotations, Annotation{
			Uuid:  annotationUuid,
			Name:  annotationName,
			Value: annotationValue,
		})
		s.SecretAnnotations = append(s.SecretAnnotations, SecretAnnotation{
			SecretUuid:     s.Uuid,
			AnnotationUuid: annotationUuid,
		})
	}
}

func (s *Secret) Relations() []database.Relation {
	fk := database.WithForeignKey("secret_uuid")

	return []database.Relation{
		database.HasMany(s.Labels, database.WithoutCascadeDelete()),
		database.HasMany(s.SecretLabels, fk),
		database.HasMany(s.SecretAnnotations, fk),
		database.HasMany(s.Annotations, database.WithoutCascadeDelete()),
	}
}
