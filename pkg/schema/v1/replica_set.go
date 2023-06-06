package v1

import (
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/strcase"
	"github.com/icinga/icinga-kubernetes/pkg/types"
	kappsv1 "k8s.io/api/apps/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
)

type ReplicaSet struct {
	Meta
	Id                   types.Binary
	DesiredReplicas      int32
	MinReadySeconds      int32
	ActualReplicas       int32
	FullyLabeledReplicas int32
	ReadyReplicas        int32
	AvailableReplicas    int32
	Conditions           []ReplicaSetCondition `db:"-"`
	Owners               []ReplicaSetOwner     `db:"-"`
}

type ReplicaSetCondition struct {
	ReplicaSetId   types.Binary
	Type           string
	Status         string
	LastTransition types.UnixMilli
	Reason         string
	Message        string
}

type ReplicaSetOwner struct {
	ReplicaSetId       types.Binary
	Kind               string
	Name               string
	Uid                ktypes.UID
	Controller         types.Bool
	BlockOwnerDeletion types.Bool
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
	r.Id = types.Checksum(r.Namespace + "/" + r.Name)
	r.DesiredReplicas = desiredReplicas
	r.MinReadySeconds = replicaSet.Spec.MinReadySeconds
	r.ActualReplicas = replicaSet.Status.Replicas
	r.FullyLabeledReplicas = replicaSet.Status.FullyLabeledReplicas
	r.ReadyReplicas = replicaSet.Status.ReadyReplicas
	r.AvailableReplicas = replicaSet.Status.AvailableReplicas

	for _, condition := range replicaSet.Status.Conditions {
		r.Conditions = append(r.Conditions, ReplicaSetCondition{
			ReplicaSetId:   r.Id,
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
			ReplicaSetId: r.Id,
			Kind:         strcase.Snake(ownerReference.Kind),
			Name:         ownerReference.Name,
			Uid:          ownerReference.UID,
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
}

func (r *ReplicaSet) Relations() database.Relations {
	return database.Relations{
		database.HasMany[ReplicaSetCondition]{
			Entities:    r.Conditions,
			ForeignKey_: "replica_set_id",
		},
		database.HasMany[ReplicaSetOwner]{
			Entities:    r.Owners,
			ForeignKey_: "pod_id",
		},
	}
}
