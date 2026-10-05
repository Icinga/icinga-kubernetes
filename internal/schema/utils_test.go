package schema

import (
	"strings"
	"testing"

	kappsv1 "k8s.io/api/apps/v1"
	kbatchv1 "k8s.io/api/batch/v1"
	kcorev1 "k8s.io/api/core/v1"
	keventsv1 "k8s.io/api/events/v1"
	knetworkingv1 "k8s.io/api/networking/v1"
	krbacv1 "k8s.io/api/rbac/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
)

// TestMarshalYaml encodes one object of every schema type that stores its YAML,
// so a type whose API group yamlEncoder doesn't register fails here instead of
// at runtime.
func TestMarshalYaml(t *testing.T) {
	meta := kmetav1.ObjectMeta{Namespace: "default", Name: "test"}

	tests := []struct {
		obj        kruntime.Object
		apiVersion string
		kind       string
	}{
		{&kbatchv1.CronJob{ObjectMeta: meta}, "batch/v1", "CronJob"},
		{&kappsv1.DaemonSet{ObjectMeta: meta}, "apps/v1", "DaemonSet"},
		{&kappsv1.Deployment{ObjectMeta: meta}, "apps/v1", "Deployment"},
		// The core group has an Event type as well, so this also covers that the
		// events.k8s.io one keeps its own group.
		{&keventsv1.Event{ObjectMeta: meta}, "events.k8s.io/v1", "Event"},
		{&knetworkingv1.Ingress{ObjectMeta: meta}, "networking.k8s.io/v1", "Ingress"},
		{&kbatchv1.Job{ObjectMeta: meta}, "batch/v1", "Job"},
		{&kcorev1.Namespace{ObjectMeta: meta}, "v1", "Namespace"},
		{&kcorev1.Node{ObjectMeta: meta}, "v1", "Node"},
		{&kcorev1.PersistentVolume{ObjectMeta: meta}, "v1", "PersistentVolume"},
		{&kcorev1.Pod{ObjectMeta: meta}, "v1", "Pod"},
		{&kcorev1.PersistentVolumeClaim{ObjectMeta: meta}, "v1", "PersistentVolumeClaim"},
		{&kappsv1.ReplicaSet{ObjectMeta: meta}, "apps/v1", "ReplicaSet"},
		{&kcorev1.Service{ObjectMeta: meta}, "v1", "Service"},
		{&kappsv1.StatefulSet{ObjectMeta: meta}, "apps/v1", "StatefulSet"},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			yaml := MarshalYaml(tt.obj)

			for _, want := range []string{"apiVersion: " + tt.apiVersion + "\n", "kind: " + tt.kind + "\n", "name: test\n"} {
				if !strings.Contains(yaml, want) {
					t.Errorf("YAML lacks %q:\n%s", want, yaml)
				}
			}
		})
	}

	t.Run("UnregisteredGroupPanics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("MarshalYaml did not panic")
			}
		}()

		MarshalYaml(&krbacv1.ClusterRole{ObjectMeta: meta})
	})
}
