package v1

import (
	b64 "encoding/base64"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	kserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"strings"
)

type Secret struct {
	Meta
	Type              string
	Immutable         types.Bool
	Yaml              string
	Data              []Data             `db:"-"`
	SecretData        []SecretData       `db:"-"`
	Labels            []Label            `db:"-"`
	SecretLabels      []SecretLabel      `db:"-"`
	Annotations       []Annotation       `db:"-"`
	SecretAnnotations []SecretAnnotation `db:"-"`
}

type SecretData struct {
	SecretUuid types.UUID
	DataUuid   types.UUID
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

	for dataName, b64dataValue := range secret.Data {
		var value string
		dataValue := make([]byte, b64.StdEncoding.DecodedLen(len(b64dataValue)))
		n, err := b64.StdEncoding.Decode(dataValue, b64dataValue)
		if err != nil {
			value = string(b64dataValue)
		} else {
			value = string(dataValue[:n])
		}

		dataUuid := NewUUID(s.Uuid, strings.ToLower(dataName+":"+value))
		s.Data = append(s.Data, Data{
			Uuid:  dataUuid,
			Name:  dataName,
			Value: value,
		})
		s.SecretData = append(s.SecretData, SecretData{
			SecretUuid: s.Uuid,
			DataUuid:   dataUuid,
		})
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

	scheme := kruntime.NewScheme()
	_ = kcorev1.AddToScheme(scheme)
	codec := kserializer.NewCodecFactory(scheme).EncoderForVersion(kjson.NewYAMLSerializer(kjson.DefaultMetaFactory, scheme, scheme), kcorev1.SchemeGroupVersion)
	output, _ := kruntime.Encode(codec, secret)
	s.Yaml = string(output)
}

func (s *Secret) Relations() []database.Relation {
	fk := database.WithForeignKey("secret_uuid")

	return []database.Relation{
		database.HasMany(s.Labels, database.WithoutCascadeDelete()),
		database.HasMany(s.SecretLabels, fk),
		database.HasMany(s.Data, database.WithoutCascadeDelete()),
		database.HasMany(s.SecretData, fk),
		database.HasMany(s.SecretAnnotations, fk),
		database.HasMany(s.Annotations, database.WithoutCascadeDelete()),
	}
}
