package notifications

// Marshaler is the interface implemented by types that can marshal themselves
// into valid notification events.
type Marshaler interface {
	MarshalEvent() (Event, error)
}

// Deleter is the interface implemented by types that can tell whether their
// object is being deleted.
type Deleter interface {
	IsDeleting() bool
}
