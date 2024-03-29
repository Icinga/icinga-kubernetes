package v1

import (
	"database/sql"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/strcase"
	"github.com/icinga/icinga-kubernetes/pkg/types"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

type Service struct {
	Meta
	Id                            types.Binary
	Type                          string
	ClusterIP                     string
	ClusterIPs                    string
	ExternalIPs                   string
	SessionAffinity               string
	ExternalName                  string
	ExternalTrafficPolicy         sql.NullString
	HealthCheckNodePort           int32
	PublishNotReadyAddresses      types.Bool
	IpFamilies                    string
	IpFamilyPolicy                string
	AllocateLoadBalancerNodePorts types.Bool
	LoadBalancerClass             string
	InternalTrafficPolicy         string
	Selectors                     []Selector         `db:"-"`
	ServiceSelectors              []ServiceSelector  `db:"-"`
	Ports                         []ServicePort      `db:"-"`
	Conditions                    []ServiceCondition `db:"-"`
	Labels                        []Label            `db:"-"`
	ServiceLabels                 []ServiceLabel     `db:"-"`
}

type ServiceSelector struct {
	ServiceId  types.Binary
	SelectorId types.Binary
}

type ServicePort struct {
	ServiceId   types.Binary
	Name        string
	Protocol    string
	AppProtocol string
	Port        int32
	TargetPort  string
	NodePort    int32
}

type ServiceCondition struct {
	ServiceId          types.Binary
	Type               string
	Status             string
	ObservedGeneration int64
	LastTransition     types.UnixMilli
	Reason             string
	Message            string
}

type ServiceLabel struct {
	ServiceId types.Binary
	LabelId   types.Binary
}

func NewService() Resource {
	return &Service{}
}

func (s *Service) Obtain(k8s kmetav1.Object) {
	s.ObtainMeta(k8s)

	service := k8s.(*kcorev1.Service)

	var externalTrafficPolicy sql.NullString
	if service.Spec.ExternalTrafficPolicy != "" {
		externalTrafficPolicy.String = strcase.Snake(string(service.Spec.ExternalTrafficPolicy))
		externalTrafficPolicy.Valid = true
	}
	var ipFamilyPolicy string
	if service.Spec.IPFamilyPolicy != nil {
		ipFamilyPolicy = strcase.Snake(string(*service.Spec.IPFamilyPolicy))
	}
	var allocateLoadBalancerNodePorts bool
	if service.Spec.AllocateLoadBalancerNodePorts != nil {
		allocateLoadBalancerNodePorts = *service.Spec.AllocateLoadBalancerNodePorts
	}
	var loadBalancerClass string
	if service.Spec.LoadBalancerClass != nil {
		loadBalancerClass = *service.Spec.LoadBalancerClass
	}
	var internalTrafficPolicy string
	if service.Spec.InternalTrafficPolicy == nil {
		internalTrafficPolicy = "cluster"
	} else {
		internalTrafficPolicy = strcase.Snake(string(*service.Spec.InternalTrafficPolicy))
	}

	s.Id = types.Checksum(service.Namespace + "/" + service.Name)
	s.Type = strcase.Snake(string(service.Spec.Type))
	s.ClusterIP = service.Spec.ClusterIP
	for _, clusterIP := range service.Spec.ClusterIPs {
		s.ClusterIPs = clusterIP
	}
	for _, externalIPs := range service.Spec.ExternalIPs {
		s.ExternalIPs = externalIPs
	}
	s.SessionAffinity = strcase.Snake(string(service.Spec.SessionAffinity))
	s.ExternalName = service.Spec.ExternalName
	s.ExternalTrafficPolicy = externalTrafficPolicy
	s.HealthCheckNodePort = service.Spec.HealthCheckNodePort
	s.PublishNotReadyAddresses = types.Bool{
		Bool:  service.Spec.PublishNotReadyAddresses,
		Valid: true,
	}
	for _, ipFamily := range service.Spec.IPFamilies {
		s.IpFamilies = string(ipFamily)
	}
	s.IpFamilyPolicy = ipFamilyPolicy
	s.AllocateLoadBalancerNodePorts = types.Bool{
		Bool:  allocateLoadBalancerNodePorts,
		Valid: true,
	}
	s.LoadBalancerClass = loadBalancerClass
	s.InternalTrafficPolicy = internalTrafficPolicy

	for selectorName, selectorValue := range service.Spec.Selector {
		selectorId := types.Checksum(strings.ToLower(selectorName + ":" + selectorValue))
		s.Selectors = append(s.Selectors, Selector{
			Id:    selectorId,
			Name:  selectorName,
			Value: selectorValue,
		})
		s.ServiceSelectors = append(s.ServiceSelectors, ServiceSelector{
			ServiceId:  s.Id,
			SelectorId: selectorId,
		})
	}

	for _, port := range service.Spec.Ports {
		var appProtocol string
		if port.AppProtocol != nil {
			appProtocol = *port.AppProtocol
		}
		s.Ports = append(s.Ports, ServicePort{
			ServiceId:   s.Id,
			Name:        port.Name,
			Protocol:    string(port.Protocol),
			AppProtocol: appProtocol,
			Port:        port.Port,
			TargetPort:  port.TargetPort.String(),
			NodePort:    port.NodePort,
		})
	}

	for _, condition := range service.Status.Conditions {
		s.Conditions = append(s.Conditions, ServiceCondition{
			ServiceId:          s.Id,
			Type:               condition.Type,
			Status:             strcase.Snake(string(condition.Status)),
			ObservedGeneration: condition.ObservedGeneration,
			LastTransition:     types.UnixMilli(condition.LastTransitionTime.Time),
			Reason:             condition.Reason,
			Message:            condition.Message,
		})
	}

	for labelName, labelValue := range service.Labels {
		labelId := types.Checksum(strings.ToLower(labelName + ":" + labelValue))
		s.Labels = append(s.Labels, Label{
			Id:    labelId,
			Name:  labelName,
			Value: labelValue,
		})
		s.ServiceLabels = append(s.ServiceLabels, ServiceLabel{
			ServiceId: s.Id,
			LabelId:   labelId,
		})
	}
}

func (s *Service) Relations() []database.Relation {
	fk := database.WithForeignKey("service_id")

	return []database.Relation{
		database.HasMany(s.Conditions, fk),
		database.HasMany(s.Ports, fk),
		database.HasMany(s.Labels, database.WithoutCascadeDelete()),
		database.HasMany(s.ServiceLabels, fk),
		database.HasMany(s.Selectors, database.WithoutCascadeDelete()),
		database.HasMany(s.ServiceSelectors, fk),
	}
}
