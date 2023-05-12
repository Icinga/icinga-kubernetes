package v1

import (
	"fmt"

	eventv1 "k8s.io/api/events/v1"
)

type Event struct {
	Name                string `db:"name"`
	Namespace           string `db:"namespace"`
	UID                 string `db:"uid"`
	ReportingController string `db:"reporting_controller"`
	ReportingInstance   string `db:"reporting_instance"`
	Action              string `db:"action"`
	Reason              string `db:"reason"`
	Note                string `db:"note"`
	Type                string `db:"type"`
	ReferenceKind       string `db:"reference_kind"`
	Reference           string `db:"reference"`
}

func NewEventFromK8s(obj *eventv1.Event) Event {
	return Event{
		Name:                obj.Name,
		Namespace:           obj.Namespace,
		UID:                 string(obj.UID),
		ReportingController: obj.ReportingController,
		ReportingInstance:   obj.ReportingInstance,
		Action:              obj.Action,
		Reason:              obj.Reason,
		Note:                obj.Note,
		Type:                obj.Type,
		ReferenceKind:       obj.Regarding.Kind,
		Reference:           fmt.Sprintf("%s/%s", obj.Regarding.Namespace, obj.Regarding.Name),
	}

}
