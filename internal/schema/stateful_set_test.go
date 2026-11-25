package schema

import (
	"testing"

	kappsv1 "k8s.io/api/apps/v1"
)

func TestStatefulSetIcingaState(t *testing.T) {
	tests := []struct {
		name      string
		desired   int32
		available int32
		want      IcingaState
	}{
		{"scaled to zero", 0, 0, Ok},
		{"none available", 2, 0, Critical},
		{"partially available", 2, 1, Warning},
		{"all available", 2, 2, Ok},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statefulSet := &kappsv1.StatefulSet{
				Namespace: "default",
				Name:      "db",
				UID:       "stateful-set-uid",
				Spec:      kappsv1.StatefulSetSpec{Replicas: new(tt.desired)},
				Status: kappsv1.StatefulSetStatus{
					Replicas:          tt.available,
					ReadyReplicas:     tt.available,
					AvailableReplicas: tt.available,
				},
			}

			s := NewStatefulSet().(*StatefulSet)
			s.Obtain(statefulSet, EnsureUUID("cluster-uid"))

			if s.IcingaState != tt.want {
				t.Errorf("IcingaState = %s, want %s (reason: %q)", s.IcingaState, tt.want, s.IcingaStateReason)
			}
		})
	}
}
