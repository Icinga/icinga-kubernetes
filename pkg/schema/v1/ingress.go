package v1

import (
	"database/sql"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	networkingv1 "k8s.io/api/networking/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	kserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"strings"
)

type Ingress struct {
	Meta
	Yaml                   string
	IngressTls             []IngressTls             `db:"-"`
	IngressBackendService  []IngressBackendService  `db:"-"`
	IngressBackendResource []IngressBackendResource `db:"-"`
	IngressRule            []IngressRule            `db:"-"`
	Labels                 []Label                  `db:"-"`
	IngressLabels          []IngressLabel           `db:"-"`
	ResourceLabels         []ResourceLabel          `db:"-"`
	Annotations            []Annotation             `db:"-"`
	IngressAnnotations     []IngressAnnotation      `db:"-"`
	ResourceAnnotations    []ResourceAnnotation     `db:"-"`
	Favorites              []Favorite               `db:"-"`
}

type IngressTls struct {
	IngressUuid types.UUID
	TlsHost     string
	TlsSecret   string
}

type IngressBackendService struct {
	ServiceUuid       types.UUID
	IngressUuid       types.UUID
	IngressRuleUuid   types.UUID
	ServiceName       string
	ServicePortName   string
	ServicePortNumber int32
}

type IngressBackendResource struct {
	ResourceUuid    types.UUID
	IngressUuid     types.UUID
	IngressRuleUuid types.UUID
	ApiGroup        sql.NullString
	Kind            string
	Name            string
}

type IngressRule struct {
	Uuid        types.UUID
	BackendUuid types.UUID
	IngressUuid types.UUID
	Host        sql.NullString
	Path        sql.NullString
	PathType    string
}

type IngressLabel struct {
	IngressUuid types.UUID
	LabelUuid   types.UUID
}

type IngressAnnotation struct {
	IngressUuid    types.UUID
	AnnotationUuid types.UUID
}

func NewIngress() Resource {
	return &Ingress{}
}

func (i *Ingress) Obtain(k8s kmetav1.Object, clusterUuid types.UUID) {
	i.ObtainMeta(k8s, clusterUuid)

	ingress := k8s.(*networkingv1.Ingress)

	for _, tls := range ingress.Spec.TLS {
		for _, host := range tls.Hosts {
			i.IngressTls = append(i.IngressTls, IngressTls{
				IngressUuid: i.Uuid,
				TlsHost:     host,
				TlsSecret:   tls.SecretName,
			})
		}
	}

	if ingress.Spec.DefaultBackend != nil {
		if ingress.Spec.DefaultBackend.Resource != nil {
			resourceUuid := NewUUID(i.Uuid, ingress.Spec.DefaultBackend.Resource.Kind+ingress.Spec.DefaultBackend.Resource.Name)
			var apiGroup sql.NullString
			if ingress.Spec.DefaultBackend.Resource.APIGroup != nil {
				apiGroup.String = *ingress.Spec.DefaultBackend.Resource.APIGroup
				apiGroup.Valid = true
				i.IngressBackendResource = append(i.IngressBackendResource, IngressBackendResource{
					ResourceUuid: resourceUuid,
					IngressUuid:  i.Uuid,
					ApiGroup:     apiGroup,
					Kind:         ingress.Spec.DefaultBackend.Resource.Kind,
					Name:         ingress.Spec.DefaultBackend.Resource.Name,
				})
			}
		}
	}

	for _, rules := range ingress.Spec.Rules {
		if rules.IngressRuleValue.HTTP == nil {
			continue
		}

		for _, ruleValue := range rules.IngressRuleValue.HTTP.Paths {
			// It is safe to use the pointer directly here.
			pathType := string(*ruleValue.PathType)
			if ruleValue.Backend.Resource != nil {
				ingressRuleUuid := NewUUID(i.Uuid, rules.Host+ruleValue.Path+ruleValue.Backend.Resource.Name)
				resourceUuid := NewUUID(ingressRuleUuid, ruleValue.Backend.Resource.Name)
				var apiGroup sql.NullString
				if ruleValue.Backend.Resource.APIGroup != nil {
					apiGroup.String = *ruleValue.Backend.Resource.APIGroup
					apiGroup.Valid = true
				}
				i.IngressBackendResource = append(i.IngressBackendResource, IngressBackendResource{
					ResourceUuid:    resourceUuid,
					IngressUuid:     i.Uuid,
					IngressRuleUuid: ingressRuleUuid,
					ApiGroup:        apiGroup,
					Kind:            ruleValue.Backend.Resource.Kind,
					Name:            ruleValue.Backend.Resource.Name,
				})
				i.IngressRule = append(i.IngressRule, IngressRule{
					Uuid:        ingressRuleUuid,
					IngressUuid: i.Uuid,
					BackendUuid: resourceUuid,
					Host:        NewNullableString(rules.Host),
					Path:        NewNullableString(ruleValue.Path),
					PathType:    pathType,
				})
			}
		}

	}

	for labelName, labelValue := range ingress.Labels {
		labelUuid := NewUUID(i.Uuid, strings.ToLower(labelName+":"+labelValue))
		i.Labels = append(i.Labels, Label{
			Uuid:  labelUuid,
			Name:  labelName,
			Value: labelValue,
		})
		i.IngressLabels = append(i.IngressLabels, IngressLabel{
			IngressUuid: i.Uuid,
			LabelUuid:   labelUuid,
		})
		i.ResourceLabels = append(i.ResourceLabels, ResourceLabel{
			ResourceUuid: i.Uuid,
			LabelUuid:    labelUuid,
		})
	}

	for annotationName, annotationValue := range ingress.Annotations {
		annotationUuid := NewUUID(i.Uuid, strings.ToLower(annotationName+":"+annotationValue))
		i.Annotations = append(i.Annotations, Annotation{
			Uuid:  annotationUuid,
			Name:  annotationName,
			Value: annotationValue,
		})
		i.IngressAnnotations = append(i.IngressAnnotations, IngressAnnotation{
			IngressUuid:    i.Uuid,
			AnnotationUuid: annotationUuid,
		})
		i.ResourceAnnotations = append(i.ResourceAnnotations, ResourceAnnotation{
			ResourceUuid:   i.Uuid,
			AnnotationUuid: annotationUuid,
		})
	}

	scheme := kruntime.NewScheme()
	_ = networkingv1.AddToScheme(scheme)
	codec := kserializer.NewCodecFactory(scheme).EncoderForVersion(kjson.NewYAMLSerializer(kjson.DefaultMetaFactory, scheme, scheme), networkingv1.SchemeGroupVersion)
	output, _ := kruntime.Encode(codec, ingress)
	i.Yaml = string(output)
}

func (i *Ingress) Relations() []database.Relation {
	fk := database.WithForeignKey("ingress_uuid")

	return []database.Relation{
		database.HasMany(i.IngressTls, fk),
		database.HasMany(i.IngressBackendService, fk),
		database.HasMany(i.IngressBackendResource, fk),
		database.HasMany(i.IngressRule, fk),
		database.HasMany(i.ResourceLabels, database.WithForeignKey("resource_uuid")),
		database.HasMany(i.Labels, database.WithoutCascadeDelete()),
		database.HasMany(i.IngressLabels, fk),
		database.HasMany(i.ResourceAnnotations, database.WithForeignKey("resource_uuid")),
		database.HasMany(i.Annotations, database.WithoutCascadeDelete()),
		database.HasMany(i.IngressAnnotations, fk),
		database.HasMany(i.Favorites, database.WithForeignKey("resource_uuid")),
	}
}
