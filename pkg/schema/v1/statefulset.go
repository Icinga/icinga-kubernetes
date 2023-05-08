package v1

import (
	appv1 "k8s.io/api/apps/v1"
)

type StatefulSet struct {
	Name              string
	Namespace         string
	UID               string
	Replicas          int32
	ServiceName       string
	ReadyReplicas     int32
	CurrentReplicas   int32
	UpdatedReplicas   int32
	AvailableReplicas int32
	CurrentRevision   string
	UpdateRevision    string
	CollisionCount    int32
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
