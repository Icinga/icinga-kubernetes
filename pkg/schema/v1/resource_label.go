package v1

import (
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
)

type ResourceLabel struct {
	ResourceUuid types.UUID
	LabelUuid    types.UUID
}

func (r ResourceLabel) ID() database.ID {
	return compoundId{id: r.ResourceUuid.String() + r.LabelUuid.String()}
}

func (r ResourceLabel) SetID(_ database.ID) {
	panic("Not expected to be called")
}

func (r ResourceLabel) Fingerprint() database.Fingerprinter {
	return r
}
