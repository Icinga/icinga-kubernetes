package v1

import (
	b64 "encoding/base64"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-go-library/utils"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

type Secret struct {
	Meta
	Id           types.Binary
	Type         string
	Immutable    types.Bool
	Data         []Data        `db:"-"`
	SecretData   []SecretData  `db:"-"`
	Labels       []Label       `db:"-"`
	SecretLabels []SecretLabel `db:"-"`
}

type SecretData struct {
	SecretId types.Binary
	DataId   types.Binary
}

type SecretLabel struct {
	SecretId types.Binary
	LabelId  types.Binary
}

func NewSecret() Resource {
	return &Secret{}
}

func (s *Secret) Obtain(k8s kmetav1.Object) {
	s.ObtainMeta(k8s)

	secret := k8s.(*kcorev1.Secret)

	s.Id = utils.Checksum(s.Namespace + "/" + s.Name)
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

		dataId := utils.Checksum(dataName + ":" + value)
		s.Data = append(s.Data, Data{
			Id:    dataId,
			Name:  dataName,
			Value: value,
		})
		s.SecretData = append(s.SecretData, SecretData{
			SecretId: s.Id,
			DataId:   dataId,
		})
	}

	for labelName, labelValue := range secret.Labels {
		labelId := utils.Checksum(strings.ToLower(labelName + ":" + labelValue))
		s.Labels = append(s.Labels, Label{
			Id:    labelId,
			Name:  labelName,
			Value: labelValue,
		})
		s.SecretLabels = append(s.SecretLabels, SecretLabel{
			SecretId: s.Id,
			LabelId:  labelId,
		})
	}
}

func (s *Secret) Relations() []database.Relation {
	fk := database.WithForeignKey("secret_id")

	return []database.Relation{
		database.HasMany(s.Labels, database.WithoutCascadeDelete()),
		database.HasMany(s.SecretLabels, fk),
		database.HasMany(s.Data, database.WithoutCascadeDelete()),
		database.HasMany(s.SecretData, fk),
	}
}
