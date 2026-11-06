package schema

import (
	"database/sql"
	"strings"

	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/strcase"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/internal/kdatabase"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Service struct {
	Meta
	ClusterIP                     string
	ClusterIPs                    string
	Type                          string
	ExternalIPs                   types.String
	SessionAffinity               string
	ExternalName                  types.String
	ExternalTrafficPolicy         types.String
	HealthCheckNodePort           sql.NullInt32
	PublishNotReadyAddresses      types.Bool
	IpFamilies                    types.String
	IpFamilyPolicy                types.String
	AllocateLoadBalancerNodePorts types.Bool
	LoadBalancerClass             types.String
	InternalTrafficPolicy         string
	Yaml                          string
	Selectors                     []ServiceSelector  `db:"-"`
	Ports                         []ServicePort      `db:"-"`
	Conditions                    []ServiceCondition `db:"-"`
	Labels                        []Label            `db:"-"`
	Annotations                   []Annotation       `db:"-"`
}

type ServiceSelector struct {
	kdatabase.EntityStub
	Uuid        types.UUID
	ServiceUuid types.UUID
	Name        string
	Value       string
}

func (s ServiceSelector) ID() database.ID {
	return s.Uuid
}

type ServicePort struct {
	kdatabase.EntityStub
	ServiceUuid types.UUID
	Name        string
	Protocol    string
	AppProtocol string
	Port        int32
	TargetPort  string
	NodePort    int32
}

func (s ServicePort) ID() database.ID {
	return compoundId{id: s.ServiceUuid.String() + s.Name}
}

type ServiceCondition struct {
	kdatabase.EntityStub
	ServiceUuid        types.UUID
	Type               string
	Status             string
	ObservedGeneration int64
	LastTransition     types.UnixMilli
	Reason             string
	Message            string
}

func (s ServiceCondition) ID() database.ID {
	return compoundId{id: s.ServiceUuid.String() + s.Type}
}

type ServicePod struct {
	kdatabase.EntityStub
	ServiceUuid types.UUID
	PodUuid     types.UUID
}

func (s ServicePod) ID() database.ID {
	return compoundId{id: s.ServiceUuid.String() + s.PodUuid.String()}
}

func NewService() Resource {
	return &Service{}
}

func (s *Service) Obtain(k8s kmetav1.Object, clusterUuid types.UUID) {
	s.ObtainMeta(k8s, clusterUuid)

	service := k8s.(*kcorev1.Service)

	for _, condition := range service.Status.Conditions {
		s.Conditions = append(s.Conditions, ServiceCondition{
			ServiceUuid:        s.Uuid,
			Type:               condition.Type,
			Status:             strcase.Snake(string(condition.Status)),
			ObservedGeneration: condition.ObservedGeneration,
			LastTransition:     types.UnixMilli(condition.LastTransitionTime.Time),
			Reason:             condition.Reason,
			Message:            condition.Message,
		})
	}

	for labelName, labelValue := range service.Labels {
		labelUuid := NewUUID(s.Uuid, strings.ToLower(labelName+":"+labelValue))
		s.Labels = append(s.Labels, Label{
			Uuid:         labelUuid,
			ResourceUuid: s.Uuid,
			Name:         labelName,
			Value:        labelValue,
		})
	}

	for annotationName, annotationValue := range service.Annotations {
		annotationUuid := NewUUID(s.Uuid, strings.ToLower(annotationName+":"+annotationValue))
		s.Annotations = append(s.Annotations, Annotation{
			Uuid:         annotationUuid,
			ResourceUuid: s.Uuid,
			Name:         annotationName,
			Value:        annotationValue,
		})
	}

	for _, port := range service.Spec.Ports {
		var appProtocol string
		if port.AppProtocol != nil {
			appProtocol = *port.AppProtocol
		}
		s.Ports = append(s.Ports, ServicePort{
			ServiceUuid: s.Uuid,
			Name:        port.Name,
			Protocol:    string(port.Protocol),
			AppProtocol: appProtocol,
			Port:        port.Port,
			TargetPort:  port.TargetPort.String(),
			NodePort:    port.NodePort,
		})
	}

	for selectorName, selectorValue := range service.Spec.Selector {
		selectorUuid := NewUUID(s.Uuid, strings.ToLower(selectorName+":"+selectorValue))
		s.Selectors = append(s.Selectors, ServiceSelector{
			Uuid:        selectorUuid,
			ServiceUuid: s.Uuid,
			Name:        selectorName,
			Value:       selectorValue,
		})
	}

	s.ClusterIP = service.Spec.ClusterIP
	s.ClusterIPs = strings.Join(service.Spec.ClusterIPs, ", ")
	s.Type = string(service.Spec.Type)
	s.ExternalIPs = types.MakeString(strings.Join(service.Spec.ExternalIPs, ", "), types.TransformEmptyStringToNull)
	s.SessionAffinity = string(service.Spec.SessionAffinity)
	// TODO(el): Support LoadBalancerSourceRanges?
	s.ExternalName = types.MakeString(service.Spec.ExternalName, types.TransformEmptyStringToNull)
	s.ExternalTrafficPolicy = types.MakeString(string(service.Spec.ExternalTrafficPolicy), types.TransformEmptyStringToNull)
	s.HealthCheckNodePort = sql.NullInt32{
		Int32: service.Spec.HealthCheckNodePort,
		Valid: service.Spec.HealthCheckNodePort != 0,
	}
	s.PublishNotReadyAddresses = types.MakeBool(service.Spec.PublishNotReadyAddresses)
	// TODO(el): Support SessionAffinityConfig?
	var ipv4 bool
	var ipv6 bool
	for _, ipFamily := range service.Spec.IPFamilies {
		switch ipFamily {
		case kcorev1.IPv4Protocol:
			ipv4 = true
		case kcorev1.IPv6Protocol:
			ipv6 = true
		}
	}
	if len(service.Spec.IPFamilies) > 0 {
		switch {
		case ipv4 && ipv6:
			s.IpFamilies = types.MakeString("DualStack")
		case ipv4:
			s.IpFamilies = types.MakeString("IPv4")
		case ipv6:
			s.IpFamilies = types.MakeString("IPv6")
		default:
			s.IpFamilies = types.MakeString("Unknown")
		}
	}
	if service.Spec.IPFamilyPolicy != nil {
		s.IpFamilyPolicy = types.MakeString(string(*service.Spec.IPFamilyPolicy), types.TransformEmptyStringToNull)
	}
	allocateLoadBalancerNodePorts := true
	if service.Spec.AllocateLoadBalancerNodePorts != nil {
		allocateLoadBalancerNodePorts = *service.Spec.AllocateLoadBalancerNodePorts
	}
	s.AllocateLoadBalancerNodePorts = types.MakeBool(allocateLoadBalancerNodePorts)
	if service.Spec.LoadBalancerClass != nil {
		s.LoadBalancerClass = types.MakeString(*service.Spec.LoadBalancerClass)
	}
	var internalTrafficPolicy string
	if service.Spec.InternalTrafficPolicy == nil {
		internalTrafficPolicy = string(kcorev1.ServiceInternalTrafficPolicyCluster)
	} else {
		internalTrafficPolicy = string(*service.Spec.InternalTrafficPolicy)
	}
	s.InternalTrafficPolicy = internalTrafficPolicy
	s.Yaml = MarshalYaml(service)
}

func (s *Service) Relations() []kdatabase.Relation {
	fk := kdatabase.WithForeignKey("service_uuid")

	return []kdatabase.Relation{
		kdatabase.HasMany(s.Conditions, fk),
		kdatabase.HasMany(s.Ports, fk),
		kdatabase.HasMany(s.Selectors, fk),
		kdatabase.HasMany(s.Labels, kdatabase.WithForeignKey("resource_uuid")),
		kdatabase.HasMany(s.Annotations, kdatabase.WithForeignKey("resource_uuid")),

		// Delete-only relations.
		kdatabase.HasMany([]Favorite(nil), kdatabase.WithForeignKey("resource_uuid")),
		kdatabase.HasMany([]ServicePod(nil), fk),
	}
}
