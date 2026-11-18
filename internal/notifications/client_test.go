package notifications

import (
	"testing"

	"github.com/icinga/icinga-go-library/notifications/event"
)

type testEntity struct {
	event    *event.Event
	deleting bool
}

func (e testEntity) MarshalEvent() *event.Event {
	return e.event
}

func (e testEntity) IsDeleting() bool {
	return e.deleting
}

func TestSuppressWhileDeleting(t *testing.T) {
	tests := []struct {
		name     string
		severity event.Severity
		deleting bool
		want     bool
	}{
		{name: "NotDeletingCritical", severity: event.SeverityCritical, deleting: false, want: false},
		{name: "NotDeletingOK", severity: event.SeverityOK, deleting: false, want: false},
		{name: "DeletingCritical", severity: event.SeverityCritical, deleting: true, want: true},
		{name: "DeletingError", severity: event.SeverityError, deleting: true, want: true},
		{name: "DeletingOK", severity: event.SeverityOK, deleting: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := &event.Event{Name: "default: web", Severity: tt.severity}

			if got := suppressWhileDeleting(testEntity{event: ev, deleting: tt.deleting}, ev); got != tt.want {
				t.Fatalf("expected %t, got %t", tt.want, got)
			}
		})
	}
}
