package schema

import (
	"strings"
	"testing"

	kcorev1 "k8s.io/api/core/v1"
)

func TestPodIcingaStateRunningContainers(t *testing.T) {
	spec := kcorev1.PodSpec{
		InitContainers: []kcorev1.Container{{Name: "migrate", Image: "shop:1.4.2"}},
		Containers: []kcorev1.Container{
			{Name: "web", Image: "shop:1.4.2"},
			{Name: "metrics", Image: "exporter:0.9.0"},
		},
	}
	running := func(name string) kcorev1.ContainerStatus {
		return kcorev1.ContainerStatus{
			Name:    name,
			Ready:   true,
			Started: new(true),
			State:   kcorev1.ContainerState{Running: &kcorev1.ContainerStateRunning{}},
		}
	}

	tests := []struct {
		name   string
		status kcorev1.PodStatus
		want   string
	}{
		{
			// Before the kubelet reports any status, every container state is unknown.
			name:   "created",
			status: kcorev1.PodStatus{Phase: kcorev1.PodPending},
			want:   "Pod default/shop-7d9f8b6c5d-x2k8q is unknown with 0 out of 2 containers running.",
		},
		{
			name: "running",
			status: kcorev1.PodStatus{
				Phase: kcorev1.PodRunning,
				Conditions: []kcorev1.PodCondition{
					{Type: kcorev1.PodScheduled, Status: kcorev1.ConditionTrue},
					{Type: kcorev1.PodInitialized, Status: kcorev1.ConditionTrue},
					{Type: kcorev1.ContainersReady, Status: kcorev1.ConditionTrue},
					{Type: kcorev1.PodReady, Status: kcorev1.ConditionTrue},
				},
				InitContainerStatuses: []kcorev1.ContainerStatus{{
					Name:  "migrate",
					State: kcorev1.ContainerState{Terminated: &kcorev1.ContainerStateTerminated{Reason: "Completed"}},
				}},
				ContainerStatuses: []kcorev1.ContainerStatus{running("web"), running("metrics")},
			},
			want: "Pod default/shop-7d9f8b6c5d-x2k8q is ok with 2 out of 2 containers running.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &kcorev1.Pod{
				Namespace: "default",
				Name:      "shop-7d9f8b6c5d-x2k8q",
				UID:       "pod-uid",
				Spec:      spec,
				Status:    tt.status,
			}

			p := NewPodFactory(nil).New().(*Pod)
			p.Obtain(pod, EnsureUUID("cluster-uid"))

			if firstLine, _, _ := strings.Cut(p.IcingaStateReason, "\n"); firstLine != tt.want {
				t.Errorf("IcingaStateReason starts with %q, want %q", firstLine, tt.want)
			}
		})
	}
}
