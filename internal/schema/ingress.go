package schema

import (
	"database/sql"
	"strings"

	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/internal/kdatabase"
	networkingv1 "k8s.io/api/networking/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Ingress struct {
	Meta
	Yaml                   string
	IngressTls             []IngressTls             `db:"-"`
	IngressBackendService  []IngressBackendService  `db:"-"`
	IngressBackendResource []IngressBackendResource `db:"-"`
	IngressRule            []IngressRule            `db:"-"`
	Labels                 []Label                  `db:"-"`
	Annotations            []Annotation             `db:"-"`
	Favorites              []Favorite               `db:"-"`
}

type IngressTls struct {
	kdatabase.EntityStub
	IngressUuid types.UUID
	TlsHost     string
	TlsSecret   string
}

func (i IngressTls) ID() database.ID {
	return i.IngressUuid
}

type IngressBackendService struct {
	kdatabase.EntityStub
	ServiceUuid       types.UUID
	IngressUuid       types.UUID
	IngressRuleUuid   types.UUID
	ServiceName       string
	ServicePortName   string
	ServicePortNumber int32
}

func (i IngressBackendService) ID() database.ID {
	return compoundId{id: i.ServiceUuid.String() + i.IngressUuid.String()}
}

type IngressBackendResource struct {
	kdatabase.EntityStub
	ResourceUuid    types.UUID
	IngressUuid     types.UUID
	IngressRuleUuid types.UUID
	ApiGroup        sql.NullString
	Kind            string
	Name            string
}

func (i IngressBackendResource) ID() database.ID {
	return compoundId{id: i.ResourceUuid.String() + i.IngressUuid.String()}
}

type IngressRule struct {
	kdatabase.EntityStub
	Uuid        types.UUID
	BackendUuid types.UUID
	IngressUuid types.UUID
	Host        sql.NullString
	Path        sql.NullString
	PathType    string
}

func (i IngressRule) ID() database.ID {
	return i.Uuid
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
		if ingress.Spec.DefaultBackend.Service != nil {
			serviceUuid := NewUUID(i.Uuid, ingress.Spec.DefaultBackend.Service.Name+ingress.Spec.DefaultBackend.Service.Port.Name)
			i.IngressBackendService = append(i.IngressBackendService, IngressBackendService{
				ServiceUuid:       serviceUuid,
				IngressUuid:       i.Uuid,
				ServiceName:       ingress.Spec.DefaultBackend.Service.Name,
				ServicePortName:   ingress.Spec.DefaultBackend.Service.Port.Name,
				ServicePortNumber: ingress.Spec.DefaultBackend.Service.Port.Number,
			})
		}
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
			if ruleValue.Backend.Service != nil {
				ingressRuleUuid := NewUUID(i.Uuid, rules.Host+ruleValue.Path+ruleValue.Backend.Service.Name)
				serviceUuid := NewUUID(ingressRuleUuid, ruleValue.Backend.Service.Name)
				i.IngressBackendService = append(i.IngressBackendService, IngressBackendService{
					ServiceUuid:       serviceUuid,
					IngressUuid:       i.Uuid,
					IngressRuleUuid:   ingressRuleUuid,
					ServiceName:       ruleValue.Backend.Service.Name,
					ServicePortName:   ruleValue.Backend.Service.Port.Name,
					ServicePortNumber: ruleValue.Backend.Service.Port.Number,
				})
				i.IngressRule = append(i.IngressRule, IngressRule{
					Uuid:        ingressRuleUuid,
					BackendUuid: serviceUuid,
					IngressUuid: i.Uuid,
					Host:        NewNullableString(rules.Host),
					Path:        NewNullableString(ruleValue.Path),
					PathType:    pathType,
				})
			} else if ruleValue.Backend.Resource != nil {
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
			Uuid:         labelUuid,
			ResourceUuid: i.Uuid,
			Name:         labelName,
			Value:        labelValue,
		})
	}

	for annotationName, annotationValue := range ingress.Annotations {
		annotationUuid := NewUUID(i.Uuid, strings.ToLower(annotationName+":"+annotationValue))
		i.Annotations = append(i.Annotations, Annotation{
			Uuid:         annotationUuid,
			ResourceUuid: i.Uuid,
			Name:         annotationName,
			Value:        annotationValue,
		})
	}

	i.Yaml = MarshalYaml(ingress)
}

func (i *Ingress) Relations() []kdatabase.Relation {
	fk := kdatabase.WithForeignKey("ingress_uuid")

	return []kdatabase.Relation{
		kdatabase.HasMany(i.IngressTls, fk),
		kdatabase.HasMany(i.IngressBackendService, fk),
		kdatabase.HasMany(i.IngressBackendResource, fk),
		kdatabase.HasMany(i.IngressRule, fk),
		kdatabase.HasMany(i.Labels, kdatabase.WithForeignKey("resource_uuid")),
		kdatabase.HasMany(i.Annotations, kdatabase.WithForeignKey("resource_uuid")),
		kdatabase.HasMany(i.Favorites, kdatabase.WithForeignKey("resource_uuid")),
	}
}
