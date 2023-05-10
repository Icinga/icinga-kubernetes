package v1

import (
	appv1 "k8s.io/api/apps/v1"
)

type StatefulSet struct {
	Name              string `db:"name"`
	Namespace         string `db:"namespace"`
	UID               string `db:"uid"`
	Replicas          int32  `db:"replicas"`
	ServiceName       string `db:"service_name"`
	ReadyReplicas     int32  `db:"ready_replicas"`
	CurrentReplicas   int32  `db:"current_replicas"`
	UpdatedReplicas   int32  `db:"updated_replicas"`
	AvailableReplicas int32  `db:"available_replicas"`
	CurrentRevision   string `db:"current_revision"`
	UpdateRevision    string `db:"update_revision"`
	CollisionCount    int32  `db:"collision_count"`
}

func NewStatefulSetFromK8s(obj *appv1.StatefulSet) StatefulSet {
	var collisionCount int32
	var replicas int32

	if obj.Status.CollisionCount != nil {
		collisionCount = *obj.Status.CollisionCount
	}

	if obj.Spec.Replicas != nil {
		replicas = *obj.Spec.Replicas
	}

	return StatefulSet{
		Name:              obj.Name,
		Namespace:         obj.Namespace,
		UID:               string(obj.UID),
		Replicas:          replicas,
		ServiceName:       obj.Spec.ServiceName,
		ReadyReplicas:     obj.Status.ReadyReplicas,
		CurrentReplicas:   obj.Status.CurrentReplicas,
		UpdatedReplicas:   obj.Status.UpdatedReplicas,
		AvailableReplicas: obj.Status.AvailableReplicas,
		CurrentRevision:   obj.Status.CurrentRevision,
		UpdateRevision:    obj.Status.UpdateRevision,
		CollisionCount:    collisionCount,
	}
}
