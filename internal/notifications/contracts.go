package notifications

import "github.com/icinga/icinga-go-library/notifications/event"

// Marshaler is the interface implemented by types that can marshal themselves
// into valid notification events.
type Marshaler interface {
	MarshalEvent() *event.Event
}
