package v1

import (
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
)

type Label struct {
	Uuid  types.UUID
	Name  string
	Value string
}

func (l Label) ID() database.ID {
	return l.Uuid
}

func (l Label) SetID(_ database.ID) {
	panic("Not expected to be called")
}

func (l Label) Fingerprint() database.Fingerprinter {
	return l
}
