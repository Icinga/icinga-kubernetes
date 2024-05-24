package v1

import (
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-go-library/utils"
	keventsv1 "k8s.io/api/events/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Event struct {
	Meta
	Id                  types.Binary
	ReportingController string
	ReportingInstance   string
	Action              string
	Reason              string
	Note                string
	Type                string
	ReferenceKind       string
	ReferenceNamespace  string
	ReferenceName       string
	FirstSeen           types.UnixMilli
	LastSeen            types.UnixMilli
	Count               int32
}

func NewEvent() Resource {
	return &Event{}
}

func (e *Event) Obtain(k8s kmetav1.Object) {
	e.ObtainMeta(k8s)

	event := k8s.(*keventsv1.Event)

	e.Id = utils.Checksum(event.Namespace + "/" + event.Name)
	e.ReportingController = event.ReportingController
	e.ReportingInstance = event.ReportingInstance
	e.Action = event.Action
	e.Reason = event.Reason
	e.Note = event.Note
	e.Type = event.Type
	e.ReferenceKind = event.Regarding.Kind
	e.ReferenceNamespace = event.Regarding.Namespace
	e.ReferenceName = event.Regarding.Name
	if event.DeprecatedFirstTimestamp.Time.IsZero() {
		e.FirstSeen = types.UnixMilli(k8s.GetCreationTimestamp().Time)
	} else {
		e.FirstSeen = types.UnixMilli(event.DeprecatedFirstTimestamp.Time)
	}
	if event.DeprecatedLastTimestamp.Time.IsZero() {
		e.LastSeen = types.UnixMilli(k8s.GetCreationTimestamp().Time)
	} else {
		e.LastSeen = types.UnixMilli(event.DeprecatedLastTimestamp.Time)
	}
	e.Count = event.DeprecatedCount
	// e.FirstSeen = types.UnixMilli(event.EventTime.Time)
	// if event.Series != nil {
	// 	e.LastSeen = types.UnixMilli(event.Series.LastObservedTime.Time)
	// 	e.Count = event.Series.Count
	// } else {
	// 	e.LastSeen = e.FirstSeen
	// 	e.Count = 1
	// }
}
