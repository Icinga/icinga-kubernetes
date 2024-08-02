package v1

import (
	"fmt"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/strcase"
	kappsv1 "k8s.io/api/apps/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	kserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	ktypes "k8s.io/apimachinery/pkg/types"
	"strings"
)

type StatefulSet struct {
	Meta
	DesiredReplicas                                 int32
	ServiceName                                     string
	PodManagementPolicy                             string
	UpdateStrategy                                  string
	MinReadySeconds                                 int32
	PersistentVolumeClaimRetentionPolicyWhenDeleted string
	PersistentVolumeClaimRetentionPolicyWhenScaled  string
	Ordinals                                        int32
	ActualReplicas                                  int32
	ReadyReplicas                                   int32
	CurrentReplicas                                 int32
	UpdatedReplicas                                 int32
	AvailableReplicas                               int32
	Yaml                                            string
	IcingaState                                     IcingaState
	IcingaStateReason                               string
	Conditions                                      []StatefulSetCondition  `db:"-"`
	Owners                                          []StatefulSetOwner      `db:"-"`
	Labels                                          []Label                 `db:"-"`
	StatefulSetLabels                               []StatefulSetLabel      `db:"-"`
	Annotations                                     []Annotation            `db:"-"`
	StatefulSetAnnotations                          []StatefulSetAnnotation `db:"-"`
}

type StatefulSetCondition struct {
	StatefulSetUuid types.UUID
	Type            string
	Status          string
	LastTransition  types.UnixMilli
	Reason          string
	Message         string
}

type StatefulSetOwner struct {
	StatefulSetUuid    types.UUID
	OwnerUuid          types.UUID
	Kind               string
	Name               string
	Uid                ktypes.UID
	Controller         types.Bool
	BlockOwnerDeletion types.Bool
}

type StatefulSetLabel struct {
	StatefulSetUuid types.UUID
	LabelUuid       types.UUID
}

type StatefulSetAnnotation struct {
	StatefulSetUuid types.UUID
	AnnotationUuid  types.UUID
}

func NewStatefulSet() Resource {
	return &StatefulSet{}
}

func (s *StatefulSet) Obtain(k8s kmetav1.Object) {
	s.ObtainMeta(k8s)

	statefulSet := k8s.(*kappsv1.StatefulSet)

	var replicas, ordinals int32
	if statefulSet.Spec.Replicas != nil {
		replicas = *statefulSet.Spec.Replicas
	}
	if statefulSet.Spec.Ordinals != nil {
		ordinals = statefulSet.Spec.Ordinals.Start
	}
	var pvcRetentionPolicyDeleted, pvcRetentionPolicyScaled kappsv1.PersistentVolumeClaimRetentionPolicyType
	if statefulSet.Spec.PersistentVolumeClaimRetentionPolicy != nil {
		pvcRetentionPolicyDeleted = statefulSet.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted
		pvcRetentionPolicyScaled = statefulSet.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled
	} else {
		pvcRetentionPolicyDeleted, pvcRetentionPolicyScaled = kappsv1.RetainPersistentVolumeClaimRetentionPolicyType, kappsv1.RetainPersistentVolumeClaimRetentionPolicyType
	}

	s.DesiredReplicas = replicas
	s.ServiceName = statefulSet.Spec.ServiceName
	s.PodManagementPolicy = strcase.Snake(string(statefulSet.Spec.PodManagementPolicy))
	s.UpdateStrategy = strcase.Snake(string(statefulSet.Spec.UpdateStrategy.Type))
	s.MinReadySeconds = statefulSet.Spec.MinReadySeconds
	s.PersistentVolumeClaimRetentionPolicyWhenDeleted = strcase.Snake(string(pvcRetentionPolicyDeleted))
	s.PersistentVolumeClaimRetentionPolicyWhenScaled = strcase.Snake(string(pvcRetentionPolicyScaled))
	s.Ordinals = ordinals
	s.ActualReplicas = statefulSet.Status.Replicas

	s.ReadyReplicas = statefulSet.Status.ReadyReplicas
	s.CurrentReplicas = statefulSet.Status.CurrentReplicas
	s.UpdatedReplicas = statefulSet.Status.UpdatedReplicas
	s.AvailableReplicas = statefulSet.Status.AvailableReplicas
	s.IcingaState, s.IcingaStateReason = s.getIcingaState()

	for _, condition := range statefulSet.Status.Conditions {
		s.Conditions = append(s.Conditions, StatefulSetCondition{
			StatefulSetUuid: s.Uuid,
			Type:            string(condition.Type),
			Status:          string(condition.Status),
			LastTransition:  types.UnixMilli(condition.LastTransitionTime.Time),
			Reason:          condition.Reason,
			Message:         condition.Message,
		})
	}

	for _, ownerReference := range statefulSet.OwnerReferences {
		var blockOwnerDeletion, controller bool
		if ownerReference.BlockOwnerDeletion != nil {
			blockOwnerDeletion = *ownerReference.BlockOwnerDeletion
		}
		if ownerReference.Controller != nil {
			controller = *ownerReference.Controller
		}
		s.Owners = append(s.Owners, StatefulSetOwner{
			StatefulSetUuid: s.Uuid,
			OwnerUuid:       EnsureUUID(ownerReference.UID),
			Kind:            strcase.Snake(ownerReference.Kind),
			Name:            ownerReference.Name,
			Uid:             ownerReference.UID,
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

	for labelName, labelValue := range statefulSet.Labels {
		labelUuid := NewUUID(s.Uuid, strings.ToLower(labelName+":"+labelValue))
		s.Labels = append(s.Labels, Label{
			Uuid:  labelUuid,
			Name:  labelName,
			Value: labelValue,
		})
		s.StatefulSetLabels = append(s.StatefulSetLabels, StatefulSetLabel{
			StatefulSetUuid: s.Uuid,
			LabelUuid:       labelUuid,
		})
	}

	for annotationName, annotationValue := range statefulSet.Annotations {
		annotationUuid := NewUUID(s.Uuid, strings.ToLower(annotationName+":"+annotationValue))
		s.Annotations = append(s.Annotations, Annotation{
			Uuid:  annotationUuid,
			Name:  annotationName,
			Value: annotationValue,
		})
		s.StatefulSetAnnotations = append(s.StatefulSetAnnotations, StatefulSetAnnotation{
			StatefulSetUuid: s.Uuid,
			AnnotationUuid:  annotationUuid,
		})
	}

	scheme := kruntime.NewScheme()
	_ = kappsv1.AddToScheme(scheme)
	codec := kserializer.NewCodecFactory(scheme).EncoderForVersion(kjson.NewYAMLSerializer(kjson.DefaultMetaFactory, scheme, scheme), kappsv1.SchemeGroupVersion)
	output, _ := kruntime.Encode(codec, statefulSet)
	s.Yaml = string(output)
}

func (s *StatefulSet) getIcingaState() (IcingaState, string) {
	if gracePeriodReason := IsWithinGracePeriod(s); gracePeriodReason != nil {
		return Ok, *gracePeriodReason
	}

	switch {
	case s.AvailableReplicas == 0:
		reason := fmt.Sprintf("StatefulSet %s/%s has no replica available from %d desired.", s.Namespace, s.Name, s.DesiredReplicas)

		return Critical, reason
	case s.AvailableReplicas < s.DesiredReplicas:
		reason := fmt.Sprintf("StatefulSet %s/%s only has %d out of %d desired replicas available.", s.Namespace, s.Name, s.AvailableReplicas, s.DesiredReplicas)

		return Warning, reason
	default:
		reason := fmt.Sprintf("StatefulSet %s/%s has all %d desired replicas available.", s.Namespace, s.Name, s.DesiredReplicas)

		return Ok, reason
	}
}

func (s *StatefulSet) Relations() []database.Relation {
	fk := database.WithForeignKey("stateful_set_uuid")

	return []database.Relation{
		database.HasMany(s.Conditions, fk),
		database.HasMany(s.Owners, fk),
		database.HasMany(s.StatefulSetLabels, fk),
		database.HasMany(s.Labels, database.WithoutCascadeDelete()),
		database.HasMany(s.StatefulSetAnnotations, fk),
		database.HasMany(s.Annotations, database.WithoutCascadeDelete()),
	}
}
