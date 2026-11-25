package schema

import (
	"testing"

	"github.com/icinga/icinga-go-library/types"
	kcorev1 "k8s.io/api/core/v1"
	knetworkingv1 "k8s.io/api/networking/v1"
)

func TestIngressDefaultBackendResource(t *testing.T) {
	tests := map[string]struct {
		apiGroup *string
		want     types.String
	}{
		"core group":  {nil, types.String{}},
		"named group": {new("example.com"), types.MakeString("example.com")},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			i := &Ingress{}
			i.Obtain(&knetworkingv1.Ingress{
				Name:      "ingress",
				Namespace: "default",
				UID:       "uid",
				Spec: knetworkingv1.IngressSpec{
					DefaultBackend: &knetworkingv1.IngressBackend{
						Resource: &kcorev1.TypedLocalObjectReference{
							APIGroup: tt.apiGroup,
							Kind:     "StorageBucket",
							Name:     "static-assets",
						},
					},
				},
			}, types.UUID{})

			if len(i.IngressBackendResource) != 1 {
				t.Fatalf("got %d backend resources, want 1", len(i.IngressBackendResource))
			}

			r := i.IngressBackendResource[0]
			if r.ApiGroup != tt.want {
				t.Errorf("got API group %v, want %v", r.ApiGroup, tt.want)
			}

			if r.Kind != "StorageBucket" || r.Name != "static-assets" {
				t.Errorf("got %s %s, want StorageBucket static-assets", r.Kind, r.Name)
			}
		})
	}
}
