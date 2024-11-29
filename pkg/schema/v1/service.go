package v1

import (
	"database/sql"
	"github.com/icinga/icinga-go-library/strcase"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	kserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"strings"
)

type Service struct {
	Meta
	ClusterIP                     string
	ClusterIPs                    string
	Type                          string
	ExternalIPs                   sql.NullString
	SessionAffinity               string
	ExternalName                  sql.NullString
	ExternalTrafficPolicy         sql.NullString
	HealthCheckNodePort           sql.NullInt32
	PublishNotReadyAddresses      types.Bool
	IpFamilies                    sql.NullString
	IpFamilyPolicy                sql.NullString
	AllocateLoadBalancerNodePorts types.Bool
	LoadBalancerClass             sql.NullString
	InternalTrafficPolicy         string
	Yaml                          string
	Selectors                     []Selector           `db:"-"`
	ServiceSelectors              []ServiceSelector    `db:"-"`
	Ports                         []ServicePort        `db:"-"`
	Conditions                    []ServiceCondition   `db:"-"`
	Labels                        []Label              `db:"-"`
	ServiceLabels                 []ServiceLabel       `db:"-"`
	Annotations                   []Annotation         `db:"-"`
	ServiceAnnotations            []ServiceAnnotation  `db:"-"`
	ResourceAnnotations           []ResourceAnnotation `db:"-"`
}

type ServiceSelector struct {
	ServiceUuid  types.UUID
	SelectorUuid types.UUID
}

type ServicePort struct {
	ServiceUuid types.UUID
	Name        string
	Protocol    string
	AppProtocol string
	Port        int32
	TargetPort  string
	NodePort    int32
}

type ServiceCondition struct {
	ServiceUuid        types.UUID
	Type               string
	Status             string
	ObservedGeneration int64
	LastTransition     types.UnixMilli
	Reason             string
	Message            string
}

type ServiceLabel struct {
	ServiceUuid types.UUID
	LabelUuid   types.UUID
}

type ServiceAnnotation struct {
	ServiceUuid    types.UUID
	AnnotationUuid types.UUID
}

func NewService() Resource {
	return &Service{}
}

func (s *Service) Obtain(k8s kmetav1.Object) {
	s.ObtainMeta(k8s)

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
			Uuid:  labelUuid,
			Name:  labelName,
			Value: labelValue,
		})
		s.ServiceLabels = append(s.ServiceLabels, ServiceLabel{
			ServiceUuid: s.Uuid,
			LabelUuid:   labelUuid,
		})
	}

	for annotationName, annotationValue := range service.Annotations {
		annotationUuid := NewUUID(s.Uuid, strings.ToLower(annotationName+":"+annotationValue))
		s.Annotations = append(s.Annotations, Annotation{
			Uuid:  annotationUuid,
			Name:  annotationName,
			Value: annotationValue,
		})
		s.ServiceAnnotations = append(s.ServiceAnnotations, ServiceAnnotation{
			ServiceUuid:    s.Uuid,
			AnnotationUuid: annotationUuid,
		})
		s.ResourceAnnotations = append(s.ResourceAnnotations, ResourceAnnotation{
			ResourceUuid:   s.Uuid,
			AnnotationUuid: annotationUuid,
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
		s.Selectors = append(s.Selectors, Selector{
			Uuid:  selectorUuid,
			Name:  selectorName,
			Value: selectorValue,
		})
		s.ServiceSelectors = append(s.ServiceSelectors, ServiceSelector{
			ServiceUuid:  s.Uuid,
			SelectorUuid: selectorUuid,
		})
	}

	s.ClusterIP = service.Spec.ClusterIP
	s.ClusterIPs = strings.Join(service.Spec.ClusterIPs, ", ")
	s.Type = string(service.Spec.Type)
	s.ExternalIPs = NewNullableString(strings.Join(service.Spec.ExternalIPs, ", "))
	s.SessionAffinity = string(service.Spec.SessionAffinity)
	// TODO(el): Support LoadBalancerSourceRanges?
	s.ExternalName = NewNullableString(service.Spec.ExternalName)
	s.ExternalTrafficPolicy = NewNullableString(string(service.Spec.ExternalTrafficPolicy))
	s.HealthCheckNodePort = sql.NullInt32{
		Int32: service.Spec.HealthCheckNodePort,
		Valid: service.Spec.HealthCheckNodePort != 0,
	}
	s.PublishNotReadyAddresses = types.Bool{
		Bool:  service.Spec.PublishNotReadyAddresses,
		Valid: true,
	}
	// TODO(el): Support SessionAffinityConfig?
	var ipv4 bool
	var ipv6 bool
	for _, ipFamily := range service.Spec.IPFamilies {
		s.IpFamilies.Valid = true

		if ipFamily == kcorev1.IPv4Protocol {
			ipv4 = true
		} else if ipFamily == kcorev1.IPv6Protocol {
			ipv6 = true
		}
	}
	switch {
	case ipv4 && ipv6:
		s.IpFamilies.String = "DualStack"
	case ipv4:
		s.IpFamilies.String = "IPv4"
	case ipv6:
		s.IpFamilies.String = "IPv6"
	default:
		s.IpFamilies.String = "Unknown"
	}
	s.IpFamilyPolicy = NewNullableString(service.Spec.IPFamilyPolicy)
	allocateLoadBalancerNodePorts := true
	if service.Spec.AllocateLoadBalancerNodePorts != nil {
		allocateLoadBalancerNodePorts = *service.Spec.AllocateLoadBalancerNodePorts
	}
	s.AllocateLoadBalancerNodePorts = types.Bool{
		Bool:  allocateLoadBalancerNodePorts,
		Valid: true,
	}
	s.LoadBalancerClass = NewNullableString(service.Spec.LoadBalancerClass)
	var internalTrafficPolicy string
	if service.Spec.InternalTrafficPolicy == nil {
		internalTrafficPolicy = string(kcorev1.ServiceInternalTrafficPolicyCluster)
	} else {
		internalTrafficPolicy = string(*service.Spec.InternalTrafficPolicy)
	}
	s.InternalTrafficPolicy = internalTrafficPolicy

	scheme := kruntime.NewScheme()
	_ = kcorev1.AddToScheme(scheme)
	codec := kserializer.NewCodecFactory(scheme).EncoderForVersion(kjson.NewYAMLSerializer(kjson.DefaultMetaFactory, scheme, scheme), kcorev1.SchemeGroupVersion)
	output, _ := kruntime.Encode(codec, service)
	s.Yaml = string(output)
}

func (s *Service) Relations() []database.Relation {
	fk := database.WithForeignKey("service_uuid")

	return []database.Relation{
		database.HasMany(s.Conditions, fk),
		database.HasMany(s.Ports, fk),
		database.HasMany(s.Labels, database.WithoutCascadeDelete()),
		database.HasMany(s.ServiceLabels, fk),
		database.HasMany(s.Selectors, database.WithoutCascadeDelete()),
		database.HasMany(s.ServiceSelectors, fk),
		database.HasMany(s.ServiceAnnotations, fk),
		database.HasMany(s.Annotations, database.WithoutCascadeDelete()),
		database.HasMany(s.ResourceAnnotations, fk),
	}
}
