package v1

import (
	"github.com/icinga/icinga-kubernetes/pkg/types"
	appv1 "k8s.io/api/apps/v1"
)

type Deployment struct {
	Name                string          `db:"name"`
	Namespace           string          `db:"namespace"`
	UID                 string          `db:"uid"`
	Strategy            string          `db:"strategy"`
	Paused              bool            `db:"paused"`
	Replicas            int32           `db:"replicas"`
	AvailableReplicas   int32           `db:"available_replicas"`
	ReadyReplicas       int32           `db:"ready_replicas"`
	UnavailableReplicas int32           `db:"unavailable_replicas"`
	CollisionCount      int32           `db:"collision_count"`
	Created             types.UnixMilli `db:"created"`
}

func NewDeploymentFromK8s(obj *appv1.Deployment) Deployment {
	var collisionCount int32
	var replicas int32

	if obj.Status.CollisionCount != nil {
		collisionCount = *obj.Status.CollisionCount
	} else {
		collisionCount = 0
	}

	if obj.Spec.Replicas != nil {
		replicas = *obj.Spec.Replicas
	} else {
		replicas = 0
	}

	return Deployment{
		Name:                obj.Name,
		Namespace:           obj.Namespace,
		UID:                 string(obj.UID),
		Strategy:            string(obj.Spec.Strategy.Type),
		Paused:              obj.Spec.Paused,
		Replicas:            replicas,
		AvailableReplicas:   obj.Status.AvailableReplicas,
		ReadyReplicas:       obj.Status.ReadyReplicas,
		UnavailableReplicas: obj.Status.UnavailableReplicas,
		CollisionCount:      collisionCount,
		Created:             types.UnixMilli(obj.CreationTimestamp.Time),
	}
}
