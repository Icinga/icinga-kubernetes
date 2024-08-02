package v1

import (
	"github.com/icinga/icinga-go-library/types"
	keventsv1 "k8s.io/api/events/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	kserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
)

type Event struct {
	Meta
	ReferentUuid        types.UUID
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
	Yaml                string
}

func NewEvent() Resource {
	return &Event{}
}

func (e *Event) Obtain(k8s kmetav1.Object) {
	e.ObtainMeta(k8s)

	event := k8s.(*keventsv1.Event)

	e.ReferentUuid = EnsureUUID(event.Regarding.UID)
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

	scheme := kruntime.NewScheme()
	_ = keventsv1.AddToScheme(scheme)
	codec := kserializer.NewCodecFactory(scheme).EncoderForVersion(kjson.NewYAMLSerializer(kjson.DefaultMetaFactory, scheme, scheme), keventsv1.SchemeGroupVersion)
	output, _ := kruntime.Encode(codec, event)
	e.Yaml = string(output)
	// e.FirstSeen = types.UnixMilli(event.EventTime.Time)
	// if event.Series != nil {
	// 	e.LastSeen = types.UnixMilli(event.Series.LastObservedTime.Time)
	// 	e.Count = event.Series.Count
	// } else {
	// 	e.LastSeen = e.FirstSeen
	// 	e.Count = 1
	// }
}
