package v1

import (
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/strcase"
	"github.com/icinga/icinga-kubernetes/pkg/types"
	kappsv1 "k8s.io/api/apps/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type StatefulSet struct {
	Meta
	Id                                              types.Binary
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
	Conditions                                      []StatefulSetCondition `db:"-"`
}

type StatefulSetCondition struct {
	StatefulSetId  types.Binary
	Type           string
	Status         string
	LastTransition types.UnixMilli
	Reason         string
	Message        string
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
	s.Id = types.Checksum(s.Namespace + "/" + s.Name)
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

	for _, condition := range statefulSet.Status.Conditions {
		s.Conditions = append(s.Conditions, StatefulSetCondition{
			StatefulSetId:  s.Id,
			Type:           string(condition.Type),
			Status:         string(condition.Status),
			LastTransition: types.UnixMilli(condition.LastTransitionTime.Time),
			Reason:         condition.Reason,
			Message:        condition.Message,
		})
	}
}

func (s *StatefulSet) Relations() database.Relations {
	return database.Relations{
		database.HasMany[StatefulSetCondition]{
			Entities:    s.Conditions,
			ForeignKey_: "stateful_set_id",
		},
	}
}
