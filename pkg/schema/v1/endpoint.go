package v1

import (
	"database/sql"
	"strings"

	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	kdatabase "github.com/icinga/icinga-kubernetes/pkg/database"
	v1 "k8s.io/api/core/v1"
	kdiscoveryv1 "k8s.io/api/discovery/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
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

func (e EndpointSliceLabel) ID() database.ID {
	return compoundId{id: e.EndpointSliceUuid.String() + e.LabelUuid.String()}
}

func (e EndpointSliceLabel) SetID(_ database.ID) {
	panic("Not expected to be called")
}

func (e EndpointSliceLabel) Fingerprint() database.Fingerprinter {
	return e
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

func (e Endpoint) ID() database.ID {
	return e.Uuid
}

func (e Endpoint) SetID(_ database.ID) {
	panic("Not expected to be called")
}

func (e Endpoint) Fingerprint() database.Fingerprinter {
	return e
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

func (e EndpointTargetRef) ID() database.ID {
	return e.EndpointSliceUuid
}

func (e EndpointTargetRef) SetID(_ database.ID) {
	panic("Not expected to be called")
}

func (e EndpointTargetRef) Fingerprint() database.Fingerprinter {
	return e
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

func (e *EndpointSlice) Relations() []kdatabase.Relation {
	fk := kdatabase.WithForeignKey("endpoint_slice_uuid")

	return []kdatabase.Relation{
		kdatabase.HasMany(e.Endpoints, fk),
		kdatabase.HasMany(e.Labels, kdatabase.WithoutCascadeDelete()),
		kdatabase.HasMany(e.EndpointLabels, fk),
		kdatabase.HasMany(e.EndpointTargetRefs, fk),
	}
}
