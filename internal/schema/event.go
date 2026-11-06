package schema

import (
	"github.com/icinga/icinga-go-library/types"
	keventsv1 "k8s.io/api/events/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Event struct {
	Meta
	ReferenceUuid       types.UUID
	ReportingController types.String
	ReportingInstance   types.String
	Action              types.String
	Reason              string
	Note                string
	Type                string
	ReferenceKind       string
	ReferenceNamespace  types.String
	ReferenceName       string
	FirstSeen           types.UnixMilli
	LastSeen            types.UnixMilli
	Count               int32
	Yaml                string
}

func NewEvent() Resource {
	return &Event{}
}

func (e *Event) Obtain(k8s kmetav1.Object, clusterUuid types.UUID) {
	e.ObtainMeta(k8s, clusterUuid)

	event := k8s.(*keventsv1.Event)

	e.ReferenceUuid = EnsureUUID(event.Regarding.UID)
	e.ReportingController = types.MakeString(event.ReportingController, types.TransformEmptyStringToNull)
	e.ReportingInstance = types.MakeString(event.ReportingInstance, types.TransformEmptyStringToNull)
	e.Action = types.MakeString(event.Action, types.TransformEmptyStringToNull)
	e.Reason = event.Reason
	e.Note = event.Note
	e.Type = event.Type
	e.ReferenceKind = event.Regarding.Kind
	e.ReferenceNamespace = types.MakeString(event.Regarding.Namespace, types.TransformEmptyStringToNull)
	e.ReferenceName = event.Regarding.Name

	if !event.EventTime.Time.IsZero() {
		e.FirstSeen = types.UnixMilli(event.EventTime.Time)
	} else if !event.DeprecatedFirstTimestamp.Time.IsZero() {
		e.FirstSeen = types.UnixMilli(event.DeprecatedFirstTimestamp.Time)
	} else {
		e.FirstSeen = types.UnixMilli(k8s.GetCreationTimestamp().Time)
	}

	var count int32
	var lastSeen types.UnixMilli

	if event.Series != nil {
		if !event.Series.LastObservedTime.IsZero() {
			lastSeen = types.UnixMilli(event.Series.LastObservedTime.Time)
		}

		count = event.Series.Count
	}

	if lastSeen.Time().IsZero() {
		if !event.DeprecatedLastTimestamp.IsZero() {
			lastSeen = types.UnixMilli(event.DeprecatedLastTimestamp.Time)
		} else {
			lastSeen = e.FirstSeen
		}
	}

	count = max(count, event.DeprecatedCount, 1)

	e.LastSeen = lastSeen
	e.Count = count

	e.Yaml = MarshalYaml(event)
}
