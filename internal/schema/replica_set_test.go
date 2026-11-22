package schema

import (
	"testing"

	kappsv1 "k8s.io/api/apps/v1"
	kcorev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func TestReplicaSetIcingaState(t *testing.T) {
	tests := []struct {
		name       string
		status     kappsv1.ReplicaSetStatus
		want       IcingaState
		wantReason string
	}{
		{
			name:       "available",
			status:     kappsv1.ReplicaSetStatus{Replicas: 2, AvailableReplicas: 2},
			want:       Ok,
			wantReason: "ReplicaSet default/shop-7d9f8b6c5d has all 2 desired replicas available.",
		},
		{
			name: "replica failure",
			status: kappsv1.ReplicaSetStatus{
				Replicas:          2,
				AvailableReplicas: 2,
				Conditions: []kappsv1.ReplicaSetCondition{
					{
						Type:    kappsv1.ReplicaSetReplicaFailure,
						Status:  kcorev1.ConditionTrue,
						Reason:  "FailedCreate",
						Message: `pods "shop-7d9f8b6c5d-x2k8q" is forbidden: exceeded quota: compute`,
					},
				},
			},
			want: Critical,
			wantReason: `ReplicaSet default/shop-7d9f8b6c5d has a failure condition: ` +
				`pods "shop-7d9f8b6c5d-x2k8q" is forbidden: exceeded quota: compute.`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replicaSet := &kappsv1.ReplicaSet{
				Namespace: "default",
				Name:      "shop-7d9f8b6c5d",
				UID:       "replica-set-uid",
				Spec:      kappsv1.ReplicaSetSpec{Replicas: ptr.To[int32](2)},
				Status:    tt.status,
			}

			r := NewReplicaSet().(*ReplicaSet)
			r.Obtain(replicaSet, EnsureUUID("cluster-uid"))

			if r.IcingaState != tt.want || r.IcingaStateReason != tt.wantReason {
				t.Errorf("got %s %q, want %s %q", r.IcingaState, r.IcingaStateReason, tt.want, tt.wantReason)
			}
		})
	}
}
