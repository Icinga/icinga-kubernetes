package common

import (
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/contracts"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type IdMeta struct {
	Id types.Binary
}

func (m *IdMeta) ID() database.ID {
	return m.Id
}

func (m *IdMeta) SetID(id database.ID) {
	m.Id = id.(types.Binary)
}

type CanonicalMeta struct {
	CanonicalName string
}

func (m *CanonicalMeta) GetCanonicalName() string {
	return m.CanonicalName
}

func (m *CanonicalMeta) SetCanonicalName(canonicalName string) {
	m.CanonicalName = canonicalName
}

func NewKEnvelope(key string) contracts.KEnvelope {
	return &kenvelope{
		IdMeta:        IdMeta{types.Checksum(key)},
		CanonicalMeta: CanonicalMeta{key},
	}
}

type kenvelope struct {
	IdMeta
	CanonicalMeta
}

func (k *kenvelope) KUpsert(kobject kmetav1.Object) contracts.KUpsert {
	return &kupsert{
		kenvelope: *k,
		kobject:   kobject,
	}
}

func (k *kenvelope) KDelete() contracts.KDelete {
	return k
}

type kupsert struct {
	kenvelope
	kobject kmetav1.Object
}

func (k *kupsert) KObject() kmetav1.Object {
	return k.kobject
}

// Assert interface compliance.
var (
	_ database.IDer = (*IdMeta)(nil)
)
