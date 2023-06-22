package v1

import (
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Service struct {
	Meta
	Type      string
	ClusterIP string
}

func NewService() Resource {
	return &Service{}
}

func (s *Service) Obtain(k8s kmetav1.Object) {
	s.ObtainMeta(k8s)

	service := k8s.(*kcorev1.Service)

	s.Type = string(service.Spec.Type)
	s.ClusterIP = service.Spec.ClusterIP
}
