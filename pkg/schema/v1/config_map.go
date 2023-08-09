package v1

import (
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/types"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

type ConfigMap struct {
	Meta
	Id              types.Binary
	Immutable       types.Bool
	Data            []Data           `db:"-"`
	ConfigMapsData  []ConfigMapData  `db:"-"`
	Labels          []Label          `db:"-"`
	ConfigMapLabels []ConfigMapLabel `db:"-"`
}

type ConfigMapData struct {
	ConfigMapId types.Binary
	DataId      types.Binary
}

type ConfigMapLabel struct {
	ConfigMapId types.Binary
	LabelId     types.Binary
}

func NewConfigMap() Resource {
	return &ConfigMap{}
}

func (c *ConfigMap) Obtain(k8s kmetav1.Object) {
	c.ObtainMeta(k8s)

	configMap := k8s.(*kcorev1.ConfigMap)

	c.Id = types.Checksum(configMap.Namespace + "/" + configMap.Name)

	var immutable bool
	if configMap.Immutable != nil {
		immutable = *configMap.Immutable
	}
	c.Immutable = types.Bool{
		Bool:  immutable,
		Valid: true,
	}

	for dataName, dataValue := range configMap.Data {
		dataId := types.Checksum(dataName + ":" + dataValue)
		c.Data = append(c.Data, Data{
			Id:    dataId,
			Name:  dataName,
			Value: dataValue,
		})
		c.ConfigMapsData = append(c.ConfigMapsData, ConfigMapData{
			ConfigMapId: c.Id,
			DataId:      dataId,
		})
	}

	for labelName, labelValue := range configMap.Labels {
		labelId := types.Checksum(strings.ToLower(labelName + ":" + labelValue))
		c.Labels = append(c.Labels, Label{
			Id:    labelId,
			Name:  labelName,
			Value: labelValue,
		})
		c.ConfigMapLabels = append(c.ConfigMapLabels, ConfigMapLabel{
			ConfigMapId: c.Id,
			LabelId:     labelId,
		})
	}
}

func (c *ConfigMap) Relations() []database.Relation {
	fk := database.WithForeignKey("config_map_id")

	return []database.Relation{
		database.HasMany(c.Labels, database.WithoutCascadeDelete()),
		database.HasMany(c.ConfigMapLabels, fk),
		database.HasMany(c.Data, database.WithoutCascadeDelete()),
		database.HasMany(c.ConfigMapsData, fk),
	}
}
