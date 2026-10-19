package v1

import (
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/kdatabase"
)

type ResourceAnnotation struct {
	kdatabase.EntityStub
	ResourceUuid   types.UUID
	AnnotationUuid types.UUID
}

func (r ResourceAnnotation) ID() database.ID {
	return compoundId{id: r.ResourceUuid.String() + r.AnnotationUuid.String()}
}
