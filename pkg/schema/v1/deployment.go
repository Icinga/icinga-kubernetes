package v1

import (
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/strcase"
	"github.com/icinga/icinga-kubernetes/pkg/types"
	kappsv1 "k8s.io/api/apps/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

type Deployment struct {
	Meta
	Id                      types.Binary
	DesiredReplicas         int32
	Strategy                string
	MinReadySeconds         int32
	ProgressDeadlineSeconds int32
	Paused                  types.Bool
	ActualReplicas          int32
	UpdatedReplicas         int32
	ReadyReplicas           int32
	AvailableReplicas       int32
	UnavailableReplicas     int32
	Conditions              []DeploymentCondition `db:"-"`
	Labels                  []Label               `db:"-"`
	DeploymentLabels        []DeploymentLabel     `db:"-"`
}

type DeploymentCondition struct {
	DeploymentId   types.Binary
	Type           string
	Status         string
	LastUpdate     types.UnixMilli
	LastTransition types.UnixMilli
	Reason         string
	Message        string
}

type DeploymentLabel struct {
	DeploymentId types.Binary
	LabelId      types.Binary
}

func NewDeployment() Resource {
	return &Deployment{}
}

func (d *Deployment) Obtain(k8s kmetav1.Object) {
	d.ObtainMeta(k8s)

	deployment := k8s.(*kappsv1.Deployment)

	var replicas, progressDeadlineSeconds int32
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}
	if deployment.Spec.ProgressDeadlineSeconds != nil {
		progressDeadlineSeconds = *deployment.Spec.ProgressDeadlineSeconds
	}
	d.Id = types.Checksum(deployment.Namespace + "/" + deployment.Name)
	d.DesiredReplicas = replicas
	d.Strategy = strcase.Snake(string(deployment.Spec.Strategy.Type))
	d.MinReadySeconds = deployment.Spec.MinReadySeconds
	d.ProgressDeadlineSeconds = progressDeadlineSeconds
	d.Paused = types.Bool{
		Bool:  deployment.Spec.Paused,
		Valid: true,
	}
	d.ActualReplicas = deployment.Status.Replicas
	d.UpdatedReplicas = deployment.Status.UpdatedReplicas
	d.AvailableReplicas = deployment.Status.AvailableReplicas
	d.ReadyReplicas = deployment.Status.ReadyReplicas
	d.UnavailableReplicas = deployment.Status.UnavailableReplicas

	for _, condition := range deployment.Status.Conditions {
		d.Conditions = append(d.Conditions, DeploymentCondition{
			DeploymentId:   d.Id,
			Type:           strcase.Snake(string(condition.Type)),
			Status:         string(condition.Status),
			LastUpdate:     types.UnixMilli(condition.LastUpdateTime.Time),
			LastTransition: types.UnixMilli(condition.LastTransitionTime.Time),
			Reason:         condition.Reason,
			Message:        condition.Message,
		})
	}

	for labelName, labelValue := range deployment.Labels {
		labelId := types.Checksum(strings.ToLower(labelName + ":" + labelValue))
		d.Labels = append(d.Labels, Label{
			Id:    labelId,
			Name:  labelName,
			Value: labelValue,
		})
		d.DeploymentLabels = append(d.DeploymentLabels, DeploymentLabel{
			DeploymentId: d.Id,
			LabelId:      labelId,
		})
	}
}

func (d *Deployment) Relations() database.Relations {
	return database.Relations{
		database.HasMany[DeploymentCondition]{
			Entities:    d.Conditions,
			ForeignKey_: "deployment_id",
		},
		database.HasMany[Label]{
			Entities:    d.Labels,
			ForeignKey_: "value", // TODO: This is a hack to not delete any labels.
		},
		database.HasMany[DeploymentLabel]{
			Entities:    d.DeploymentLabels,
			ForeignKey_: "deployment_id",
		},
	}
}
