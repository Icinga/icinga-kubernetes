package v1

import (
	"github.com/icinga/icinga-kubernetes/pkg/types"
	corev1 "k8s.io/api/core/v1"
)

type Service struct {
	Name      string
	Namespace string
	UID       string
	Type      string
	ClusterIP string `db:"cluster_ip"`
	Created   types.UnixMilli
}

func NewServiceFromK8s(obj *corev1.Service) (*Service, error) {
	return &Service{
		Name:      obj.Name,
		Namespace: obj.Namespace,
		UID:       string(obj.UID),
		Type:      string(obj.Spec.Type),
		ClusterIP: obj.Spec.ClusterIP,
		Created:   types.UnixMilli(obj.CreationTimestamp.Time),
	}, nil
}
