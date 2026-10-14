package v1

import (
	"database/sql"
	"strings"

	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/strcase"
	"github.com/icinga/icinga-go-library/types"
	kdatabase "github.com/icinga/icinga-kubernetes/pkg/database"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	kserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes"
)

type ServiceFactory struct {
	clientset *kubernetes.Clientset
}

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
	ResourceLabels                []ResourceLabel      `db:"-"`
	Annotations                   []Annotation         `db:"-"`
	ServiceAnnotations            []ServiceAnnotation  `db:"-"`
	ResourceAnnotations           []ResourceAnnotation `db:"-"`
	ServicePods                   []ServicePod         `db:"-"`
	Favorites                     []Favorite           `db:"-"`
	factory                       *ServiceFactory
}

type ServiceSelector struct {
	ServiceUuid  types.UUID
	SelectorUuid types.UUID
}

func (s ServiceSelector) ID() database.ID {
	return compoundId{id: s.ServiceUuid.String() + s.SelectorUuid.String()}
}

func (s ServiceSelector) SetID(_ database.ID) {
	panic("Not expected to be called")
}

func (s ServiceSelector) Fingerprint() database.Fingerprinter {
	return s
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

func (s ServicePort) ID() database.ID {
	return compoundId{id: s.ServiceUuid.String() + s.Name}
}

func (s ServicePort) SetID(_ database.ID) {
	panic("Not expected to be called")
}

func (s ServicePort) Fingerprint() database.Fingerprinter {
	return s
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

func (s ServiceCondition) ID() database.ID {
	return compoundId{id: s.ServiceUuid.String() + s.Type}
}

func (s ServiceCondition) SetID(_ database.ID) {
	panic("Not expected to be called")
}

func (s ServiceCondition) Fingerprint() database.Fingerprinter {
	return s
}

type ServiceLabel struct {
	ServiceUuid types.UUID
	LabelUuid   types.UUID
}

func (s ServiceLabel) ID() database.ID {
	return compoundId{id: s.ServiceUuid.String() + s.LabelUuid.String()}
}

func (s ServiceLabel) SetID(_ database.ID) {
	panic("Not expected to be called")
}

func (s ServiceLabel) Fingerprint() database.Fingerprinter {
	return s
}

type ServiceAnnotation struct {
	ServiceUuid    types.UUID
	AnnotationUuid types.UUID
}

func (s ServiceAnnotation) ID() database.ID {
	return compoundId{id: s.ServiceUuid.String() + s.AnnotationUuid.String()}
}

func (s ServiceAnnotation) SetID(_ database.ID) {
	panic("Not expected to be called")
}

func (s ServiceAnnotation) Fingerprint() database.Fingerprinter {
	return s
}

type ServicePod struct {
	ServiceUuid types.UUID
	PodUuid     types.UUID
}

func (s ServicePod) ID() database.ID {
	return compoundId{id: s.ServiceUuid.String() + s.PodUuid.String()}
}

func (s ServicePod) SetID(_ database.ID) {
	panic("Not expected to be called")
}

func (s ServicePod) Fingerprint() database.Fingerprinter {
	return s
}

func NewServiceFactory(clientset *kubernetes.Clientset) *ServiceFactory {
	return &ServiceFactory{
		clientset: clientset,
	}
}

func (f *ServiceFactory) NewService() Resource {
	return &Service{factory: f}
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
			Uuid:  labelUuid,
			Name:  labelName,
			Value: labelValue,
		})
		s.ServiceLabels = append(s.ServiceLabels, ServiceLabel{
			ServiceUuid: s.Uuid,
			LabelUuid:   labelUuid,
		})
		s.ResourceLabels = append(s.ResourceLabels, ResourceLabel{
			ResourceUuid: s.Uuid,
			LabelUuid:    labelUuid,
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

func (s *Service) Relations() []kdatabase.Relation {
	fk := kdatabase.WithForeignKey("service_uuid")

	return []kdatabase.Relation{
		kdatabase.HasMany(s.Conditions, fk),
		kdatabase.HasMany(s.Ports, fk),
		kdatabase.HasMany(s.Selectors, kdatabase.WithoutCascadeDelete()),
		kdatabase.HasMany(s.ServiceSelectors, fk),
		kdatabase.HasMany(s.ResourceLabels, kdatabase.WithForeignKey("resource_uuid")),
		kdatabase.HasMany(s.Labels, kdatabase.WithoutCascadeDelete()),
		kdatabase.HasMany(s.ServiceLabels, fk),
		kdatabase.HasMany(s.ResourceAnnotations, kdatabase.WithForeignKey("resource_uuid")),
		kdatabase.HasMany(s.Annotations, kdatabase.WithoutCascadeDelete()),
		kdatabase.HasMany(s.ServiceAnnotations, fk),
		kdatabase.HasMany(s.ServicePods, fk),
		kdatabase.HasMany(s.Favorites, kdatabase.WithForeignKey("resource_uuid")),
	}
}
