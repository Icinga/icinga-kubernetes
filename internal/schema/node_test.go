package schema

import (
	"testing"

	kcorev1 "k8s.io/api/core/v1"
)

func TestNodeIcingaState(t *testing.T) {
	networkUnavailable := kcorev1.NodeCondition{Type: kcorev1.NodeNetworkUnavailable, Status: kcorev1.ConditionTrue}
	memoryPressureUnknown := kcorev1.NodeCondition{Type: kcorev1.NodeMemoryPressure, Status: kcorev1.ConditionUnknown}
	readyUnknown := kcorev1.NodeCondition{Type: kcorev1.NodeReady, Status: kcorev1.ConditionUnknown}
	readyFalse := kcorev1.NodeCondition{Type: kcorev1.NodeReady, Status: kcorev1.ConditionFalse}
	readyTrue := kcorev1.NodeCondition{Type: kcorev1.NodeReady, Status: kcorev1.ConditionTrue}

	tests := []struct {
		name       string
		conditions []kcorev1.NodeCondition
		want       IcingaState
	}{
		{"ready", []kcorev1.NodeCondition{memoryPressureUnknown, readyTrue}, Ok},
		{"no conditions", nil, Ok},
		{"not ready", []kcorev1.NodeCondition{readyFalse}, Critical},
		{"ready unknown", []kcorev1.NodeCondition{memoryPressureUnknown, readyUnknown}, Unknown},
		// The node lifecycle controller sets the kubelet conditions to Unknown when a node stops
		// reporting, but keeps NetworkUnavailable, which can precede Ready.
		{"network unavailable before ready unknown", []kcorev1.NodeCondition{networkUnavailable, readyUnknown}, Critical},
		{"ready unknown before network unavailable", []kcorev1.NodeCondition{readyUnknown, networkUnavailable}, Critical},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &kcorev1.Node{
				Name:   "worker-1",
				UID:    "node-uid",
				Status: kcorev1.NodeStatus{Conditions: tt.conditions},
			}

			n := NewNode().(*Node)
			n.Obtain(node, EnsureUUID("cluster-uid"))

			if n.IcingaState != tt.want {
				t.Errorf("IcingaState = %s, want %s (reason: %q)", n.IcingaState, tt.want, n.IcingaStateReason)
			}
		})
	}
}
