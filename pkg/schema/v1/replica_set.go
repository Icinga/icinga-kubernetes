package v1

import (
	"fmt"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/strcase"
	kappsv1 "k8s.io/api/apps/v1"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	kserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	ktypes "k8s.io/apimachinery/pkg/types"
	"strings"
)

type ReplicaSet struct {
	Meta
	DesiredReplicas       int32
	MinReadySeconds       int32
	ActualReplicas        int32
	FullyLabeledReplicas  int32
	ReadyReplicas         int32
	AvailableReplicas     int32
	Yaml                  string
	IcingaState           IcingaState
	IcingaStateReason     string
	Conditions            []ReplicaSetCondition  `db:"-"`
	Owners                []ReplicaSetOwner      `db:"-"`
	Labels                []Label                `db:"-"`
	ReplicaSetLabels      []ReplicaSetLabel      `db:"-"`
	Annotations           []Annotation           `db:"-"`
	ReplicaSetAnnotations []ReplicaSetAnnotation `db:"-"`
}

type ReplicaSetCondition struct {
	ReplicaSetUuid types.UUID
	Type           string
	Status         string
	LastTransition types.UnixMilli
	Reason         string
	Message        string
}

type ReplicaSetOwner struct {
	ReplicaSetUuid     types.UUID
	OwnerUuid          types.UUID
	Kind               string
	Name               string
	Uid                ktypes.UID
	Controller         types.Bool
	BlockOwnerDeletion types.Bool
}

type ReplicaSetLabel struct {
	ReplicaSetUuid types.UUID
	LabelUuid      types.UUID
}

type ReplicaSetAnnotation struct {
	ReplicaSetUuid types.UUID
	AnnotationUuid types.UUID
}

func NewReplicaSet() Resource {
	return &ReplicaSet{}
}

func (r *ReplicaSet) Obtain(k8s kmetav1.Object) {
	r.ObtainMeta(k8s)

	replicaSet := k8s.(*kappsv1.ReplicaSet)

	var desiredReplicas int32
	if replicaSet.Spec.Replicas != nil {
		desiredReplicas = *replicaSet.Spec.Replicas
	}
	r.DesiredReplicas = desiredReplicas
	r.MinReadySeconds = replicaSet.Spec.MinReadySeconds
	r.ActualReplicas = replicaSet.Status.Replicas
	r.FullyLabeledReplicas = replicaSet.Status.FullyLabeledReplicas
	r.ReadyReplicas = replicaSet.Status.ReadyReplicas
	r.AvailableReplicas = replicaSet.Status.AvailableReplicas
	r.IcingaState, r.IcingaStateReason = r.getIcingaState()

	for _, condition := range replicaSet.Status.Conditions {
		r.Conditions = append(r.Conditions, ReplicaSetCondition{
			ReplicaSetUuid: r.Uuid,
			Type:           string(condition.Type),
			Status:         string(condition.Status),
			LastTransition: types.UnixMilli(condition.LastTransitionTime.Time),
			Reason:         condition.Reason,
			Message:        condition.Message,
		})
	}

	for _, ownerReference := range replicaSet.OwnerReferences {
		var blockOwnerDeletion, controller bool
		if ownerReference.BlockOwnerDeletion != nil {
			blockOwnerDeletion = *ownerReference.BlockOwnerDeletion
		}
		if ownerReference.Controller != nil {
			controller = *ownerReference.Controller
		}
		r.Owners = append(r.Owners, ReplicaSetOwner{
			ReplicaSetUuid: r.Uuid,
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

	for labelName, labelValue := range replicaSet.Labels {
		labelUuid := NewUUID(r.Uuid, strings.ToLower(labelName+":"+labelValue))
		r.Labels = append(r.Labels, Label{
			Uuid:  labelUuid,
			Name:  labelName,
			Value: labelValue,
		})
		r.ReplicaSetLabels = append(r.ReplicaSetLabels, ReplicaSetLabel{
			ReplicaSetUuid: r.Uuid,
			LabelUuid:      labelUuid,
		})
	}

	for annotationName, annotationValue := range replicaSet.Annotations {
		annotationUuid := NewUUID(r.Uuid, strings.ToLower(annotationName+":"+annotationValue))
		r.Annotations = append(r.Annotations, Annotation{
			Uuid:  annotationUuid,
			Name:  annotationName,
			Value: annotationValue,
		})
		r.ReplicaSetAnnotations = append(r.ReplicaSetAnnotations, ReplicaSetAnnotation{
			ReplicaSetUuid: r.Uuid,
			AnnotationUuid: annotationUuid,
		})
	}

	scheme := kruntime.NewScheme()
	_ = kappsv1.AddToScheme(scheme)
	codec := kserializer.NewCodecFactory(scheme).EncoderForVersion(kjson.NewYAMLSerializer(kjson.DefaultMetaFactory, scheme, scheme), kappsv1.SchemeGroupVersion)
	output, _ := kruntime.Encode(codec, replicaSet)
	r.Yaml = string(output)
}

func (r *ReplicaSet) getIcingaState() (IcingaState, string) {
	if r.DesiredReplicas < 1 {
		reason := fmt.Sprintf("ReplicaSet %s/%s has an invalid desired replica count: %d.", r.Namespace, r.Name, r.DesiredReplicas)

		return Unknown, reason
	}

	if gracePeriodReason := IsWithinGracePeriod(r); gracePeriodReason != nil {
		return Ok, *gracePeriodReason
	}

	for _, condition := range r.Conditions {
		if condition.Type == string(kappsv1.ReplicaSetReplicaFailure) && condition.Status == string(kcorev1.ConditionTrue) {
			reason := fmt.Sprintf("ReplicaSet %s/%s has a failure condition: %s.", r.Namespace, r.Name, condition.Message)

			return Critical, reason
		}
	}

	switch {
	case r.AvailableReplicas < 1:
		reason := fmt.Sprintf("ReplicaSet %s/%s has no replica available from %d desired.", r.Namespace, r.Name, r.DesiredReplicas)

		return Critical, reason
	case r.AvailableReplicas < r.DesiredReplicas:
		reason := fmt.Sprintf("ReplicaSet %s/%s only has %d out of %d desired replicas available.", r.Namespace, r.Name, r.AvailableReplicas, r.DesiredReplicas)

		return Warning, reason
	default:
		reason := fmt.Sprintf("ReplicaSet %s/%s has all %d desired replicas available.", r.Namespace, r.Name, r.DesiredReplicas)

		return Ok, reason
	}
}

func (r *ReplicaSet) Relations() []database.Relation {
	fk := database.WithForeignKey("replica_set_uuid")

	return []database.Relation{
		database.HasMany(r.Conditions, fk),
		database.HasMany(r.Owners, fk),
		database.HasMany(r.ReplicaSetLabels, fk),
		database.HasMany(r.Labels, database.WithoutCascadeDelete()),
		database.HasMany(r.ReplicaSetAnnotations, fk),
		database.HasMany(r.Annotations, database.WithoutCascadeDelete()),
	}
}
