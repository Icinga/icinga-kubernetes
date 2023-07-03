package contracts

import (
	"github.com/icinga/icinga-kubernetes/pkg/types"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Entity is implemented by every Icinga k8s type.
// It just encapsulates other essential interfaces for an entity but doesn't have its own methods.
type Entity interface {
	IDer
	ParentIDer
	Checksumer
	FingerPrinter
}

// FingerPrinter is implemented by every Icinga k8s type.
type FingerPrinter interface {
	// Fingerprint returns the columns of this type, which are retrieved from the
	// database during the initial config dump and are used for the config delta and for caching.
	Fingerprint() FingerPrinter
}

// Checksumer is implemented by every Icinga k8s type that maintains its own database table.
type Checksumer interface {
	// Checksum computes and returns the sha1 value of this type.
	Checksum() types.Binary
}

// IDer is implemented by every Icinga k8s type that provides a unique identifier.
type IDer interface {
	// ID returns the unique identifier of this entity as a binary.
	ID() types.Binary
	SetID(id types.Binary)
}

// ParentIDer is implemented by every Icinga k8s type that provides a unique parent identifier.
// This is a no-op for all types by default. Currently, it's only implemented by all entities of
// a k8s entity sub resources.
type ParentIDer interface {
	// ParentID returns the parent id of this entity.
	ParentID() types.Binary
}

type Resource interface {
	kmetav1.Object
	Entity

	Obtain(k8s kmetav1.Object)
}

type Meta struct {
	Id                 types.Binary `db:"id"`
	PropertiesChecksum types.Binary `hash:"-"`
}

func (m *Meta) Checksum() types.Binary {
	return m.PropertiesChecksum
}

func (m *Meta) ID() types.Binary {
	return m.Id
}

func (m *Meta) SetID(id types.Binary) {
	m.Id = id
}

func (m *Meta) Fingerprint() FingerPrinter {
	return m
}

func (m *Meta) ParentID() types.Binary {
	return nil
}

// Assert interface compliance.
var (
	_ FingerPrinter = (*Meta)(nil)
	_ Checksumer    = (*Meta)(nil)
	_ IDer          = (*Meta)(nil)
	_ ParentIDer    = (*Meta)(nil)
	_ Entity        = (*Meta)(nil)
)
