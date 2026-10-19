package database

import "github.com/icinga/icinga-go-library/database"

// EntityStub provides the SetID and Fingerprint methods of database.Entity,
// which neither we nor icinga-go-library call. Embed it so that an entity only
// has to implement ID, which splitting bulk upserts on duplicate IDs needs.
// It adds no columns.
type EntityStub struct{}

func (EntityStub) SetID(database.ID) {
	panic("Not expected to be called")
}

func (EntityStub) Fingerprint() database.Fingerprinter {
	panic("Not expected to be called")
}
