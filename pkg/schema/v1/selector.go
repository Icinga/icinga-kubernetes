package v1

import (
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
)

type Selector struct {
	Uuid  types.UUID
	Name  string
	Value string
}

func (s Selector) ID() database.ID {
	return s.Uuid
}

func (s Selector) SetID(_ database.ID) {
	panic("Not expected to be called")
}

func (s Selector) Fingerprint() database.Fingerprinter {
	return s
}
