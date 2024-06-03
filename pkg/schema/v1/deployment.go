package v1

import (
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/strcase"
	kappsv1 "k8s.io/api/apps/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	kserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"strings"
)

type Deployment struct {
	Meta
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
	Yaml                    string
	Conditions              []DeploymentCondition `db:"-"`
	Labels                  []Label               `db:"-"`
	DeploymentLabels        []DeploymentLabel     `db:"-"`
}

type DeploymentCondition struct {
	DeploymentUuid types.UUID
	Type           string
	Status         string
	LastUpdate     types.UnixMilli
	LastTransition types.UnixMilli
	Reason         string
	Message        string
}

type DeploymentLabel struct {
	DeploymentUuid types.UUID
	LabelUuid      types.UUID
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
			DeploymentUuid: d.Uuid,
			Type:           strcase.Snake(string(condition.Type)),
			Status:         string(condition.Status),
			LastUpdate:     types.UnixMilli(condition.LastUpdateTime.Time),
			LastTransition: types.UnixMilli(condition.LastTransitionTime.Time),
			Reason:         condition.Reason,
			Message:        condition.Message,
		})
	}

	for labelName, labelValue := range deployment.Labels {
		labelUuid := NewUUID(d.Uuid, strings.ToLower(labelName+":"+labelValue))
		d.Labels = append(d.Labels, Label{
			Uuid:  labelUuid,
			Name:  labelName,
			Value: labelValue,
		})
		d.DeploymentLabels = append(d.DeploymentLabels, DeploymentLabel{
			DeploymentUuid: d.Uuid,
			LabelUuid:      labelUuid,
		})
	}

	scheme := kruntime.NewScheme()
	_ = kappsv1.AddToScheme(scheme)
	codec := kserializer.NewCodecFactory(scheme).EncoderForVersion(kjson.NewYAMLSerializer(kjson.DefaultMetaFactory, scheme, scheme), kappsv1.SchemeGroupVersion)
	output, _ := kruntime.Encode(codec, deployment)
	d.Yaml = string(output)
}

func (d *Deployment) Relations() []database.Relation {
	fk := database.WithForeignKey("deployment_uuid")

	return []database.Relation{
		database.HasMany(d.Conditions, fk),
		database.HasMany(d.DeploymentLabels, fk),
		database.HasMany(d.Labels, database.WithoutCascadeDelete()),
	}
}
