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

type ConfigMap struct {
	Meta
	Immutable            types.Bool
	Yaml                 string
	Data                 []Data                `db:"-"`
	ConfigMapsData       []ConfigMapData       `db:"-"`
	Labels               []Label               `db:"-"`
	ConfigMapLabels      []ConfigMapLabel      `db:"-"`
	Annotations          []Annotation          `db:"-"`
	ConfigMapAnnotations []ConfigMapAnnotation `db:"-"`
}

type ConfigMapData struct {
	ConfigMapUuid types.UUID
	DataUuid      types.UUID
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

	for dataName, dataValue := range configMap.Data {
		dataUuid := NewUUID(c.Uuid, strings.ToLower(dataName+":"+dataValue))
		c.Data = append(c.Data, Data{
			Uuid:  dataUuid,
			Name:  dataName,
			Value: dataValue,
		})
		c.ConfigMapsData = append(c.ConfigMapsData, ConfigMapData{
			ConfigMapUuid: c.Uuid,
			DataUuid:      dataUuid,
		})
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
	}

	scheme := kruntime.NewScheme()
	_ = kcorev1.AddToScheme(scheme)
	codec := kserializer.NewCodecFactory(scheme).EncoderForVersion(kjson.NewYAMLSerializer(kjson.DefaultMetaFactory, scheme, scheme), kcorev1.SchemeGroupVersion)
	output, _ := kruntime.Encode(codec, configMap)
	c.Yaml = string(output)
}

func (c *ConfigMap) Relations() []database.Relation {
	fk := database.WithForeignKey("config_map_uuid")

	return []database.Relation{
		database.HasMany(c.Labels, database.WithoutCascadeDelete()),
		database.HasMany(c.ConfigMapLabels, fk),
		database.HasMany(c.Data, database.WithoutCascadeDelete()),
		database.HasMany(c.ConfigMapsData, fk),
		database.HasMany(c.ConfigMapAnnotations, fk),
		database.HasMany(c.Annotations, database.WithoutCascadeDelete()),
	}
}
