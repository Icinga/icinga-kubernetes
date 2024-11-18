package notifications

// Marshaler is the interface implemented by types that
// can marshal themselves into valid notification events.
type Marshaler interface {
	MarshalEvent() (Event, error)
}
