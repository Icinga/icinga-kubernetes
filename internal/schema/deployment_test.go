package schema

import (
	"testing"

	kappsv1 "k8s.io/api/apps/v1"
	kcorev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func TestDeploymentIcingaState(t *testing.T) {
	tests := []struct {
		name       string
		status     kappsv1.DeploymentStatus
		want       IcingaState
		wantReason string
	}{
		{
			name: "available",
			status: kappsv1.DeploymentStatus{
				Replicas:          2,
				AvailableReplicas: 2,
				Conditions: []kappsv1.DeploymentCondition{
					{
						Type:    kappsv1.DeploymentAvailable,
						Status:  kcorev1.ConditionTrue,
						Reason:  "MinimumReplicasAvailable",
						Message: "Deployment has minimum availability.",
					},
				},
			},
			want:       Ok,
			wantReason: "Deployment default/shop has all 2 desired replicas available.",
		},
		{
			name: "replica failure",
			status: kappsv1.DeploymentStatus{
				Replicas:            1,
				AvailableReplicas:   1,
				UnavailableReplicas: 1,
				Conditions: []kappsv1.DeploymentCondition{
					{
						Type:    kappsv1.DeploymentAvailable,
						Status:  kcorev1.ConditionTrue,
						Reason:  "MinimumReplicasAvailable",
						Message: "Deployment has minimum availability.",
					},
					{
						Type:    kappsv1.DeploymentReplicaFailure,
						Status:  kcorev1.ConditionTrue,
						Reason:  "FailedCreate",
						Message: `pods "shop-7d9f8b6c5d-x2k8q" is forbidden: exceeded quota: compute`,
					},
				},
			},
			want: Critical,
			wantReason: `Deployment default/shop has replica failure: ` +
				`pods "shop-7d9f8b6c5d-x2k8q" is forbidden: exceeded quota: compute.`,
		},
		{
			name: "not available",
			status: kappsv1.DeploymentStatus{
				Replicas:            2,
				UnavailableReplicas: 2,
				Conditions: []kappsv1.DeploymentCondition{
					{
						Type:    kappsv1.DeploymentAvailable,
						Status:  kcorev1.ConditionFalse,
						Reason:  "MinimumReplicasUnavailable",
						Message: "Deployment does not have minimum availability",
					},
				},
			},
			want:       Critical,
			wantReason: "Deployment default/shop is not available: Deployment does not have minimum availability.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deployment := &kappsv1.Deployment{
				Namespace: "default",
				Name:      "shop",
				UID:       "deployment-uid",
				Spec: kappsv1.DeploymentSpec{
					Replicas:                ptr.To[int32](2),
					ProgressDeadlineSeconds: ptr.To[int32](600),
				},
				Status: tt.status,
			}

			d := NewDeployment().(*Deployment)
			d.Obtain(deployment, EnsureUUID("cluster-uid"))

			if d.IcingaState != tt.want || d.IcingaStateReason != tt.wantReason {
				t.Errorf("got %s %q, want %s %q", d.IcingaState, d.IcingaStateReason, tt.want, tt.wantReason)
			}
		})
	}
}
