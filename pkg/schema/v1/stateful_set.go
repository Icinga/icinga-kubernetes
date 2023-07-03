package v1

import (
	"github.com/icinga/icinga-kubernetes/pkg/contracts"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/strcase"
	"github.com/icinga/icinga-kubernetes/pkg/types"
	kappsv1 "k8s.io/api/apps/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type StatefulSet struct {
	ResourceMeta
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
	Conditions                                      []*StatefulSetCondition `db:"-" hash:"-"`
	Labels                                          []*Label                `db:"-" hash:"-"`
}

type StatefulSetMeta struct {
	contracts.Meta
	StatefulSetId types.Binary
}

func (sm *StatefulSetMeta) Fingerprint() contracts.FingerPrinter {
	return sm
}

func (sm *StatefulSetMeta) ParentID() types.Binary {
	return sm.StatefulSetId
}

type StatefulSetCondition struct {
	StatefulSetMeta
	Type           string
	Status         string
	LastTransition types.UnixMilli
	Reason         string
	Message        string
}

func NewStatefulSet() contracts.Entity {
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

	s.PropertiesChecksum = types.HashStruct(s)

	for _, condition := range statefulSet.Status.Conditions {
		cond := &StatefulSetCondition{
			StatefulSetMeta: StatefulSetMeta{
				StatefulSetId: s.Id,
				Meta:          contracts.Meta{Id: types.Checksum(types.MustPackSlice(s.Id, condition.Type))},
			},
			Type:           string(condition.Type),
			Status:         string(condition.Status),
			LastTransition: types.UnixMilli(condition.LastTransitionTime.Time),
			Reason:         condition.Reason,
			Message:        condition.Message,
		}
		cond.PropertiesChecksum = types.HashStruct(cond)

		s.Conditions = append(s.Conditions, cond)
	}

	for labelName, labelValue := range statefulSet.Labels {
		label := NewLabel(labelName, labelValue)
		label.StatefulSetId = s.Id
		label.PropertiesChecksum = types.HashStruct(label)

		s.Labels = append(s.Labels, label)
	}
}

func (s *StatefulSet) Relations() []database.Relation {
	fk := database.WithForeignKey("stateful_set_id")

	return []database.Relation{
		database.HasMany(s.Conditions, fk),
		database.HasMany(s.Labels, fk),
	}
}
