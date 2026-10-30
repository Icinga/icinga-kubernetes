package notifications

import (
	"testing"
)

type testEntity struct {
	event    Event
	deleting bool
}

func (e testEntity) MarshalEvent() Event {
	return e.event
}

func (e testEntity) IsDeleting() bool {
	return e.deleting
}

func TestMarshalEvent(t *testing.T) {
	crit := Event{
		Kind:     "stateful_set",
		Name:     "default/web",
		Severity: "crit",
		Message:  "StatefulSet default/web is critical.",
	}

	t.Run("NotDeleting", func(t *testing.T) {
		if got := marshalEvent(testEntity{event: crit}); got.Severity != crit.Severity || got.Message != crit.Message {
			t.Fatalf("expected the event unchanged, got severity %q and message %q", got.Severity, got.Message)
		}
	})

	t.Run("Deleting", func(t *testing.T) {
		got := marshalEvent(testEntity{event: crit, deleting: true})

		if want := "ok"; got.Severity != want {
			t.Fatalf("expected severity %q, got %q", want, got.Severity)
		}

		if want := "Stateful set default/web is being deleted."; got.Message != want {
			t.Fatalf("expected message %q, got %q", want, got.Message)
		}
	})
}
