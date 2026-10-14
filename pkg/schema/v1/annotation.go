package v1

import (
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
)

type Annotation struct {
	Uuid  types.UUID
	Name  string
	Value string
}

func (a Annotation) ID() database.ID {
	return a.Uuid
}

func (a Annotation) SetID(_ database.ID) {
	panic("Not expected to be called")
}

func (a Annotation) Fingerprint() database.Fingerprinter {
	return a
}
