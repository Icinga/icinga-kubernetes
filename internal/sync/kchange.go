package sync

import (
	"github.com/icinga/icinga-go-library/types"
	"go.uber.org/zap/zapcore"
)

// KChangeType is the type of change that occurred.
type KChangeType string

const (
	// KChangeUpsert is the KChangeType for an upsert event.
	KChangeUpsert KChangeType = "upsert"

	// KChangeDelete is the KChangeType for a delete event.
	KChangeDelete KChangeType = "delete"
)

// KChange is a Kubernetes object change taken from an informer event and
// queued for the worker, which resolves the object itself against the
// informer store. Its fields must stay comparable, as the workqueue keeps
// its items in maps and dedupes them by equality.
type KChange struct {
	changeType KChangeType
	uuid       types.UUID
	kkey       string
}

// NewKChange returns a KChange of the given type for the object with the
// given UUID and store key.
func NewKChange(changeType KChangeType, uuid types.UUID, kkey string) KChange {
	return KChange{
		changeType: changeType,
		uuid:       uuid,
		kkey:       kkey,
	}
}

// ChangeType returns the type of the change.
func (k KChange) ChangeType() KChangeType {
	return k.changeType
}

// Uuid returns the UUID of the change's object.
func (k KChange) Uuid() types.UUID {
	return k.uuid
}

// KKey returns the KKey of the change's object.
func (k KChange) KKey() string {
	return k.kkey
}

// MarshalLogObject implements the zapcore.ObjectMarshaler interface.
func (k KChange) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("change_type", string(k.changeType))
	enc.AddString("uuid", k.uuid.String())
	enc.AddString("k_key", k.kkey)

	return nil
}

// Assert interface compliance.
var _ zapcore.ObjectMarshaler = KChange{}
