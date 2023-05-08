package v1

import (
	"github.com/icinga/icinga-kubernetes/pkg/types"
	appv1 "k8s.io/api/apps/v1"
)

type ReplicaSet struct {
	Name                 string
	Namespace            string
	DesiredReplicas      int32 `db:"desired_replicas"`
	ActualReplicas       int32 `db:"actual_replicas"`
	MinReadySeconds      int32 `db:"min_ready_seconds"`
	FullyLabeledReplicas int32 `db:"fully_labeled_replicas"`
	ReadyReplicas        int32 `db:"ready_replicas"`
	AvailableReplicas    int32 `db:"available_replicas"`
	Created              types.UnixMilli
}

func NewReplicaSetFromK8s(obj *appv1.ReplicaSet) (*ReplicaSet, error) {
	var desiredReplicas int32
	if obj.Spec.Replicas != nil {
		desiredReplicas = *obj.Spec.Replicas
	}

	return &ReplicaSet{
		Name:                 obj.Name,
		Namespace:            obj.Namespace,
		DesiredReplicas:      desiredReplicas,
		ActualReplicas:       obj.Status.Replicas,
		MinReadySeconds:      obj.Spec.MinReadySeconds,
		FullyLabeledReplicas: obj.Status.FullyLabeledReplicas,
		ReadyReplicas:        obj.Status.ReadyReplicas,
		AvailableReplicas:    obj.Status.AvailableReplicas,
		Created:              types.UnixMilli(obj.CreationTimestamp.Time),
	}, nil
}
