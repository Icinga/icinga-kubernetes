package v1

import (
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/strcase"
	kappsv1 "k8s.io/api/apps/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	"strings"
)

type ReplicaSet struct {
	Meta
	DesiredReplicas      int32
	MinReadySeconds      int32
	ActualReplicas       int32
	FullyLabeledReplicas int32
	ReadyReplicas        int32
	AvailableReplicas    int32
	Conditions           []ReplicaSetCondition `db:"-"`
	Owners               []ReplicaSetOwner     `db:"-"`
	Labels               []Label               `db:"-"`
	ReplicaSetLabels     []ReplicaSetLabel     `db:"-"`
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

	for _, condition := range replicaSet.Status.Conditions {
		r.Conditions = append(r.Conditions, ReplicaSetCondition{
			ReplicaSetUuid: r.Uuid,
			Type:           strcase.Snake(string(condition.Type)),
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
}

func (r *ReplicaSet) Relations() []database.Relation {
	fk := database.WithForeignKey("replica_set_uuid")

	return []database.Relation{
		database.HasMany(r.Conditions, fk),
		database.HasMany(r.Owners, fk),
		database.HasMany(r.ReplicaSetLabels, fk),
		database.HasMany(r.Labels, database.WithoutCascadeDelete()),
	}
}
