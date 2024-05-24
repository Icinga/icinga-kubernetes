package v1

import (
	"database/sql"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-go-library/utils"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	networkingv1 "k8s.io/api/networking/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Ingress struct {
	Meta
	Id                     types.Binary
	IngressTls             []IngressTls             `db:"-"`
	IngressBackendService  []IngressBackendService  `db:"-"`
	IngressBackendResource []IngressBackendResource `db:"-"`
	IngressRule            []IngressRule            `db:"-"`
}

type IngressTls struct {
	IngressId types.Binary
	TlsHost   string
	TlsSecret string
}

type IngressBackendService struct {
	ServiceId         types.Binary
	IngressId         types.Binary
	IngressRuleId     types.Binary
	ServiceName       string
	ServicePortName   string
	ServicePortNumber int32
}

type IngressBackendResource struct {
	ResourceId    types.Binary
	IngressId     types.Binary
	IngressRuleId types.Binary
	ApiGroup      sql.NullString
	Kind          string
	Name          string
}

type IngressRule struct {
	Id        types.Binary
	BackendId types.Binary
	IngressId types.Binary
	Host      string
	Path      string
	PathType  sql.NullString
}

func NewIngress() Resource {
	return &Ingress{}
}

func (i *Ingress) Obtain(k8s kmetav1.Object) {
	i.ObtainMeta(k8s)

	ingress := k8s.(*networkingv1.Ingress)

	i.Id = utils.Checksum(i.Namespace + "/" + i.Name)
	for _, tls := range ingress.Spec.TLS {
		for _, host := range tls.Hosts {
			i.IngressTls = append(i.IngressTls, IngressTls{
				IngressId: i.Id,
				TlsHost:   host,
				TlsSecret: tls.SecretName,
			})
		}
	}

	if ingress.Spec.DefaultBackend != nil {
		if ingress.Spec.DefaultBackend.Service != nil {
			serviceId := utils.Checksum(i.Namespace + ingress.Spec.DefaultBackend.Service.Name + ingress.Spec.DefaultBackend.Service.Port.Name)
			i.IngressBackendService = append(i.IngressBackendService, IngressBackendService{
				ServiceId:         serviceId,
				IngressId:         i.Id,
				ServiceName:       ingress.Spec.DefaultBackend.Service.Name,
				ServicePortName:   ingress.Spec.DefaultBackend.Service.Port.Name,
				ServicePortNumber: ingress.Spec.DefaultBackend.Service.Port.Number,
			})
		}
		if ingress.Spec.DefaultBackend.Resource != nil {
			resourceId := utils.Checksum(i.Namespace + ingress.Spec.DefaultBackend.Resource.Kind + ingress.Spec.DefaultBackend.Resource.Name)
			var apiGroup sql.NullString
			if ingress.Spec.DefaultBackend.Resource.APIGroup != nil {
				apiGroup.String = *ingress.Spec.DefaultBackend.Resource.APIGroup
				apiGroup.Valid = true
				i.IngressBackendResource = append(i.IngressBackendResource, IngressBackendResource{
					ResourceId: resourceId,
					IngressId:  i.Id,
					ApiGroup:   apiGroup,
					Kind:       ingress.Spec.DefaultBackend.Resource.Kind,
					Name:       ingress.Spec.DefaultBackend.Resource.Name,
				})
			}
		}
	}

	for _, rules := range ingress.Spec.Rules {
		if rules.IngressRuleValue.HTTP == nil {
			continue
		}

		for _, ruleValue := range rules.IngressRuleValue.HTTP.Paths {
			var pathType sql.NullString
			if ruleValue.PathType != nil {
				pathType.String = string(*ruleValue.PathType)
				pathType.Valid = true
			}
			if ruleValue.Backend.Service != nil {
				ingressRuleId := utils.Checksum(string(i.Id) + rules.Host + ruleValue.Path + ruleValue.Backend.Service.Name)
				serviceId := utils.Checksum(string(ingressRuleId) + i.Namespace + ruleValue.Backend.Service.Name)
				i.IngressBackendService = append(i.IngressBackendService, IngressBackendService{
					ServiceId:         serviceId,
					IngressId:         i.Id,
					IngressRuleId:     ingressRuleId,
					ServiceName:       ruleValue.Backend.Service.Name,
					ServicePortName:   ruleValue.Backend.Service.Port.Name,
					ServicePortNumber: ruleValue.Backend.Service.Port.Number,
				})
				i.IngressRule = append(i.IngressRule, IngressRule{
					Id:        ingressRuleId,
					BackendId: serviceId,
					IngressId: i.Id,
					Host:      rules.Host,
					Path:      ruleValue.Path,
					PathType:  pathType,
				})
			} else if ruleValue.Backend.Resource != nil {
				ingressRuleId := utils.Checksum(string(i.Id) + rules.Host + ruleValue.Path + ruleValue.Backend.Resource.Name)
				resourceId := utils.Checksum(string(ingressRuleId) + i.Namespace + ruleValue.Backend.Resource.Name)
				var apiGroup sql.NullString
				if ruleValue.Backend.Resource.APIGroup != nil {
					apiGroup.String = *ruleValue.Backend.Resource.APIGroup
					apiGroup.Valid = true
				}
				i.IngressBackendResource = append(i.IngressBackendResource, IngressBackendResource{
					ResourceId:    resourceId,
					IngressId:     i.Id,
					IngressRuleId: ingressRuleId,
					ApiGroup:      apiGroup,
					Kind:          ruleValue.Backend.Resource.Kind,
					Name:          ruleValue.Backend.Resource.Name,
				})
				i.IngressRule = append(i.IngressRule, IngressRule{
					Id:        ingressRuleId,
					IngressId: i.Id,
					BackendId: resourceId,
					Host:      rules.Host,
					Path:      ruleValue.Path,
					PathType:  pathType,
				})
			}
		}

	}
}

func (i *Ingress) Relations() []database.Relation {
	fk := database.WithForeignKey("ingress_id")

	return []database.Relation{
		database.HasMany(i.IngressTls, fk),
		database.HasMany(i.IngressBackendService, fk),
		database.HasMany(i.IngressBackendResource, fk),
		database.HasMany(i.IngressRule, fk),
	}
}
