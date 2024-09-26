package v1

import (
	"fmt"
	"github.com/icinga/icinga-go-library/strcase"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	kappsv1 "k8s.io/api/apps/v1"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	kserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	ktypes "k8s.io/apimachinery/pkg/types"
	"strings"
)

type Deployment struct {
	Meta
	Strategy                string
	MinReadySeconds         int32
	ProgressDeadlineSeconds int32
	Paused                  types.Bool
	DesiredReplicas         int32
	ActualReplicas          int32
	UpdatedReplicas         int32
	ReadyReplicas           int32
	AvailableReplicas       int32
	UnavailableReplicas     int32
	Yaml                    string
	IcingaState             IcingaState
	IcingaStateReason       string
	Conditions              []DeploymentCondition  `db:"-"`
	Owners                  []DeploymentOwner      `db:"-"`
	Labels                  []Label                `db:"-"`
	DeploymentLabels        []DeploymentLabel      `db:"-"`
	Annotations             []Annotation           `db:"-"`
	DeploymentAnnotations   []DeploymentAnnotation `db:"-"`
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

type DeploymentOwner struct {
	DeploymentUuid     types.UUID
	OwnerUuid          types.UUID
	Kind               string
	Name               string
	Uid                ktypes.UID
	Controller         types.Bool
	BlockOwnerDeletion types.Bool
}

type DeploymentLabel struct {
	DeploymentUuid types.UUID
	LabelUuid      types.UUID
}

type DeploymentAnnotation struct {
	DeploymentUuid types.UUID
	AnnotationUuid types.UUID
}

func NewDeployment() Resource {
	return &Deployment{}
}

func (d *Deployment) Obtain(k8s kmetav1.Object) {
	d.ObtainMeta(k8s)

	deployment := k8s.(*kappsv1.Deployment)

	d.Strategy = string(deployment.Spec.Strategy.Type)
	d.MinReadySeconds = deployment.Spec.MinReadySeconds
	// It is safe to use the pointer directly here,
	// as Kubernetes sets it to 600s if no deadline is configured.
	d.ProgressDeadlineSeconds = *deployment.Spec.ProgressDeadlineSeconds
	d.Paused = types.Bool{
		Bool:  deployment.Spec.Paused,
		Valid: true,
	}
	// It is safe to use the pointer directly here,
	// as Kubernetes sets it to 1 if no replicas are configured.
	d.DesiredReplicas = *deployment.Spec.Replicas
	d.ActualReplicas = deployment.Status.Replicas
	d.UpdatedReplicas = deployment.Status.UpdatedReplicas
	d.AvailableReplicas = deployment.Status.AvailableReplicas
	d.ReadyReplicas = deployment.Status.ReadyReplicas
	d.UnavailableReplicas = deployment.Status.UnavailableReplicas
	d.IcingaState, d.IcingaStateReason = d.getIcingaState()

	for _, condition := range deployment.Status.Conditions {
		d.Conditions = append(d.Conditions, DeploymentCondition{
			DeploymentUuid: d.Uuid,
			Type:           string(condition.Type),
			Status:         string(condition.Status),
			LastUpdate:     types.UnixMilli(condition.LastUpdateTime.Time),
			LastTransition: types.UnixMilli(condition.LastTransitionTime.Time),
			Reason:         condition.Reason,
			Message:        condition.Message,
		})
	}

	for _, ownerReference := range deployment.OwnerReferences {
		var blockOwnerDeletion, controller bool
		if ownerReference.BlockOwnerDeletion != nil {
			blockOwnerDeletion = *ownerReference.BlockOwnerDeletion
		}
		if ownerReference.Controller != nil {
			controller = *ownerReference.Controller
		}
		d.Owners = append(d.Owners, DeploymentOwner{
			DeploymentUuid: d.Uuid,
			OwnerUuid:      EnsureUUID(ownerReference.UID),
			Kind:           strcase.Snake(ownerReference.Kind),
			Name:           ownerReference.Name,
			Uid:            ownerReference.UID,
			BlockOwnerDeletion: types.Bool{
				Bool:  blockOwnerDeletion,
				Valid: true,
			},
			Controller: types.Bool{
				Bool:  controller,
				Valid: true,
			},
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

	for annotationName, annotationValue := range deployment.Annotations {
		annotationUuid := NewUUID(d.Uuid, strings.ToLower(annotationName+":"+annotationValue))
		d.Annotations = append(d.Annotations, Annotation{
			Uuid:  annotationUuid,
			Name:  annotationName,
			Value: annotationValue,
		})
		d.DeploymentAnnotations = append(d.DeploymentAnnotations, DeploymentAnnotation{
			DeploymentUuid: d.Uuid,
			AnnotationUuid: annotationUuid,
		})
	}

	scheme := kruntime.NewScheme()
	_ = kappsv1.AddToScheme(scheme)
	codec := kserializer.NewCodecFactory(scheme).EncoderForVersion(kjson.NewYAMLSerializer(kjson.DefaultMetaFactory, scheme, scheme), kappsv1.SchemeGroupVersion)
	output, _ := kruntime.Encode(codec, deployment)
	d.Yaml = string(output)
}

func (d *Deployment) getIcingaState() (IcingaState, string) {
	for _, condition := range d.Conditions {
		if condition.Type == string(kappsv1.DeploymentAvailable) && condition.Status != string(kcorev1.ConditionTrue) {
			reason := fmt.Sprintf("Deployment %s/%s is not available: %s.", d.Namespace, d.Name, condition.Message)

			return Critical, reason
		}
		if condition.Type == string(kappsv1.ReplicaSetReplicaFailure) && condition.Status != string(kcorev1.ConditionTrue) {
			reason := fmt.Sprintf("Deployment %s/%s has replica failure: %s.", d.Namespace, d.Name, condition.Message)

			return Critical, reason
		}
	}

	switch {
	case d.UnavailableReplicas > 0:
		reason := fmt.Sprintf("Deployment %s/%s has %d unavailable replicas.", d.Namespace, d.Name, d.UnavailableReplicas)

		return Critical, reason
	case d.AvailableReplicas < d.DesiredReplicas:
		reason := fmt.Sprintf("Deployment %s/%s only has %d out of %d desired replicas available.", d.Namespace, d.Name, d.AvailableReplicas, d.DesiredReplicas)

		return Warning, reason
	default:
		reason := fmt.Sprintf("Deployment %s/%s has all %d desired replicas available.", d.Namespace, d.Name, d.DesiredReplicas)

		return Ok, reason
	}
}

func (d *Deployment) Relations() []database.Relation {
	fk := database.WithForeignKey("deployment_uuid")

	return []database.Relation{
		database.HasMany(d.Conditions, fk),
		database.HasMany(d.Owners, fk),
		database.HasMany(d.DeploymentLabels, fk),
		database.HasMany(d.Labels, database.WithoutCascadeDelete()),
		database.HasMany(d.DeploymentAnnotations, fk),
		database.HasMany(d.Annotations, database.WithoutCascadeDelete()),
	}
}
