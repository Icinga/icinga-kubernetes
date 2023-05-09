package v1

import (
	"github.com/icinga/icinga-kubernetes/pkg/types"
	appv1 "k8s.io/api/apps/v1"
)

type DaemonSet struct {
	Name                   string
	Namespace              string
	UID                    string
	MinReadySeconds        int32 `db:"min_ready_seconds"`
	CurrentNumberScheduled int32 `db:"current_number_scheduled"`
	NumberMisscheduled     int32 `db:"number_misscheduled"`
	DesiredNumberScheduled int32 `db:"desired_number_scheduled"`
	NumberReady            int32 `db:"number_ready"`
	CollisionCount         int32 `db:"collision_count"`
	Created                types.UnixMilli
}

func NewDaemonSetFromK8s(obj *appv1.DaemonSet) (*DaemonSet, error) {
	var collisionCount int32
	if obj.Status.CollisionCount != nil {
		collisionCount = *obj.Status.CollisionCount
	}

	return &DaemonSet{
		Name:                   obj.Name,
		Namespace:              obj.Namespace,
		UID:                    string(obj.UID),
		MinReadySeconds:        obj.Spec.MinReadySeconds,
		CurrentNumberScheduled: obj.Status.CurrentNumberScheduled,
		NumberMisscheduled:     obj.Status.NumberMisscheduled,
		DesiredNumberScheduled: obj.Status.DesiredNumberScheduled,
		NumberReady:            obj.Status.NumberReady,
		CollisionCount:         collisionCount,
		Created:                types.UnixMilli(obj.CreationTimestamp.Time),
	}, nil
}
