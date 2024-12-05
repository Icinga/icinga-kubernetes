package v1

import (
	"database/sql"
	"github.com/icinga/icinga-go-library/types"
	keventsv1 "k8s.io/api/events/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	kserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
)

type Event struct {
	Meta
	ReferenceUuid       types.UUID
	ReportingController sql.NullString
	ReportingInstance   sql.NullString
	Action              sql.NullString
	Reason              string
	Note                string
	Type                string
	ReferenceKind       string
	ReferenceNamespace  sql.NullString
	ReferenceName       string
	FirstSeen           types.UnixMilli
	LastSeen            types.UnixMilli
	Count               int32
	Yaml                string
}

func NewEvent() Resource {
	return &Event{}
}

func (e *Event) Obtain(k8s kmetav1.Object) {
	e.ObtainMeta(k8s)

	event := k8s.(*keventsv1.Event)

	e.ReferenceUuid = EnsureUUID(event.Regarding.UID)
	e.ReportingController = NewNullableString(event.ReportingController)
	e.ReportingInstance = NewNullableString(event.ReportingInstance)
	e.Action = NewNullableString(event.Action)
	e.Reason = event.Reason
	e.Note = event.Note
	e.Type = event.Type
	e.ReferenceKind = event.Regarding.Kind
	e.ReferenceNamespace = NewNullableString(event.Regarding.Namespace)
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

	scheme := kruntime.NewScheme()
	_ = keventsv1.AddToScheme(scheme)
	codec := kserializer.NewCodecFactory(scheme).EncoderForVersion(kjson.NewYAMLSerializer(kjson.DefaultMetaFactory, scheme, scheme), keventsv1.SchemeGroupVersion)
	output, _ := kruntime.Encode(codec, event)
	e.Yaml = string(output)
}
