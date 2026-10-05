package v1

import (
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/internal/kdatabase"
)

type Annotation struct {
	kdatabase.EntityStub
	Uuid         types.UUID
	ResourceUuid types.UUID
	Name         string
	Value        string
}

func (a Annotation) ID() database.ID {
	return a.Uuid
}
