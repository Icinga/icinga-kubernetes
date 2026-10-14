package v1

import (
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
)

type ResourceAnnotation struct {
	ResourceUuid   types.UUID
	AnnotationUuid types.UUID
}

func (r ResourceAnnotation) ID() database.ID {
	return compoundId{id: r.ResourceUuid.String() + r.AnnotationUuid.String()}
}

func (r ResourceAnnotation) SetID(_ database.ID) {
	panic("Not expected to be called")
}

func (r ResourceAnnotation) Fingerprint() database.Fingerprinter {
	return r
}
