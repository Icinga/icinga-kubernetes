package v1

import (
	"database/sql"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	v1 "k8s.io/api/core/v1"
	kdiscoveryv1 "k8s.io/api/discovery/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	"strings"
)

type EndpointSlice struct {
	Meta
	AddressType        string
	Endpoints          []Endpoint           `db:"-"`
	Labels             []Label              `db:"-"`
	EndpointLabels     []EndpointSliceLabel `db:"-"`
	EndpointTargetRefs []EndpointTargetRef  `db:"-"`
}

type EndpointSliceLabel struct {
	EndpointSliceUuid types.UUID
	LabelUuid         types.UUID
}

type Endpoint struct {
	Uuid              types.UUID
	EndpointSliceUuid types.UUID
	HostName          string
	NodeName          string
	Ready             types.Bool
	Serving           types.Bool
	Terminating       types.Bool
	Address           string
	PortName          string
	Protocol          string
	Port              int32
	AppProtocol       string
}

type EndpointTargetRef struct {
	EndpointSliceUuid types.UUID
	Kind              sql.NullString
	Namespace         string
	Name              string
	Uid               ktypes.UID
	ApiVersion        string
	ResourceVersion   string
}

func NewEndpointSlice() Resource {
	return &EndpointSlice{}
}

func (e *EndpointSlice) Obtain(k8s kmetav1.Object, clusterUuid types.UUID) {
	e.ObtainMeta(k8s, clusterUuid)

	endpointSlice := k8s.(*kdiscoveryv1.EndpointSlice)

	e.AddressType = string(endpointSlice.AddressType)

	for labelName, labelValue := range endpointSlice.Labels {
		labelUuid := NewUUID(e.Uuid, strings.ToLower(labelName+":"+labelValue))
		e.Labels = append(e.Labels, Label{
			Uuid:  labelUuid,
			Name:  labelName,
			Value: labelValue,
		})
		e.EndpointLabels = append(e.EndpointLabels, EndpointSliceLabel{
			EndpointSliceUuid: e.Uuid,
			LabelUuid:         labelUuid,
		})
	}

	for _, endpoint := range endpointSlice.Endpoints {
		var hostName, nodeName string
		if endpoint.Hostname != nil {
			hostName = *endpoint.Hostname
		}
		if endpoint.NodeName != nil {
			nodeName = *endpoint.NodeName
		}
		var ready, serving, terminating types.Bool
		if endpoint.Conditions.Ready != nil {
			ready.Bool = *endpoint.Conditions.Ready
			ready.Valid = true
		}
		if endpoint.Conditions.Serving != nil {
			serving.Bool = *endpoint.Conditions.Serving
			serving.Valid = true
		}
		if endpoint.Conditions.Terminating != nil {
			terminating.Bool = *endpoint.Conditions.Terminating
			terminating.Valid = true
		}
		for _, endpointPort := range endpointSlice.Ports {
			var name, protocol, appProtocol string
			var port int32
			if endpointPort.Name != nil {
				name = *endpointPort.Name
			}
			if endpointPort.Protocol != nil {
				protocol = string(*endpointPort.Protocol)
			}
			if endpointPort.Port != nil {
				port = *endpointPort.Port
			}
			if endpointPort.AppProtocol != nil {
				appProtocol = *endpointPort.AppProtocol
			}
			for _, address := range endpoint.Addresses {
				endpointUuid := NewUUID(e.Uuid, name+address+string(port))
				e.Endpoints = append(e.Endpoints, Endpoint{
					Uuid:              endpointUuid,
					EndpointSliceUuid: e.Uuid,
					HostName:          hostName,
					NodeName:          nodeName,
					Ready:             ready,
					Serving:           serving,
					Terminating:       terminating,
					PortName:          name,
					Protocol:          protocol,
					Port:              port,
					AppProtocol:       appProtocol,
					Address:           address,
				})
			}
		}
		var targetRef v1.ObjectReference
		if endpoint.TargetRef != nil {
			targetRef = *endpoint.TargetRef
		}
		var kind sql.NullString
		if targetRef.Kind != "" {
			kind.String = targetRef.Kind
			kind.Valid = true
		}
		e.EndpointTargetRefs = append(e.EndpointTargetRefs, EndpointTargetRef{
			EndpointSliceUuid: e.Uuid,
			Kind:              kind,
			Namespace:         targetRef.Namespace,
			Name:              targetRef.Name,
			Uid:               targetRef.UID,
			ApiVersion:        targetRef.APIVersion,
			ResourceVersion:   targetRef.ResourceVersion,
		})
	}
}

func (e *EndpointSlice) Relations() []database.Relation {
	fk := database.WithForeignKey("endpoint_slice_uuid")

	return []database.Relation{
		database.HasMany(e.Endpoints, fk),
		database.HasMany(e.Labels, database.WithoutCascadeDelete()),
		database.HasMany(e.EndpointLabels, fk),
		database.HasMany(e.EndpointTargetRefs, fk),
	}
}
