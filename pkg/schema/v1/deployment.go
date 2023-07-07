package v1

import (
	"github.com/icinga/icinga-kubernetes/pkg/contracts"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/strcase"
	"github.com/icinga/icinga-kubernetes/pkg/types"
	kappsv1 "k8s.io/api/apps/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Deployment struct {
	ResourceMeta
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
	Conditions              []*DeploymentCondition `db:"-" hash:"-"`
	Labels                  []*Label               `db:"-" hash:"-"`
}

type DeploymentConditionMeta struct {
	contracts.Meta
	DeploymentId types.Binary
}

func (dm *DeploymentConditionMeta) Fingerprint() contracts.FingerPrinter {
	return dm
}

func (dm *DeploymentConditionMeta) ParentID() types.Binary {
	return dm.DeploymentId
}

type DeploymentCondition struct {
	DeploymentConditionMeta
	Type           string
	Status         string
	LastUpdate     types.UnixMilli
	LastTransition types.UnixMilli
	Reason         string
	Message        string
}

func NewDeployment() contracts.Entity {
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

	d.PropertiesChecksum = types.HashStruct(d)

	for _, condition := range deployment.Status.Conditions {
		deploymentCond := &DeploymentCondition{
			DeploymentConditionMeta: DeploymentConditionMeta{
				DeploymentId: d.Id,
				Meta:         contracts.Meta{Id: types.Checksum(types.MustPackSlice(d.Id, condition.Type))},
			},
			Type:           strcase.Snake(string(condition.Type)),
			Status:         string(condition.Status),
			LastUpdate:     types.UnixMilli(condition.LastUpdateTime.Time),
			LastTransition: types.UnixMilli(condition.LastTransitionTime.Time),
			Reason:         condition.Reason,
			Message:        condition.Message,
		}
		deploymentCond.PropertiesChecksum = types.HashStruct(deploymentCond)

		d.Conditions = append(d.Conditions, deploymentCond)
	}

	for labelName, labelValue := range deployment.Labels {
		label := NewLabel(labelName, labelValue)
		label.DeploymentId = d.Id
		label.PropertiesChecksum = types.HashStruct(label)

		d.Labels = append(d.Labels, label)
	}
}

func (d *Deployment) Relations() []database.Relation {
	fk := database.WithForeignKey("deployment_id")

	return []database.Relation{
		database.HasMany(d.Conditions, fk),
		database.HasMany(d.Labels, fk),
	}
}
