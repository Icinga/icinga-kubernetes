package v1

import (
	"github.com/icinga/icinga-kubernetes/pkg/contracts"
	"github.com/icinga/icinga-kubernetes/pkg/types"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Service struct {
	ResourceMeta
	Type      string
	ClusterIP string
}

func NewService() contracts.Entity {
	return &Service{}
}

func (s *Service) Obtain(k8s kmetav1.Object) {
	s.ObtainMeta(k8s)

	service := k8s.(*kcorev1.Service)

	s.Id = types.Checksum(s.Namespace + "/" + s.Name)
	s.Type = string(service.Spec.Type)
	s.ClusterIP = service.Spec.ClusterIP
	s.PropertiesChecksum = types.Checksum(MustMarshalJSON(s))
}
