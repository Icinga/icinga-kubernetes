package schema

import (
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/internal/kdatabase"
)

type Label struct {
	kdatabase.EntityStub
	Uuid         types.UUID
	ResourceUuid types.UUID
	Name         string
	Value        string
}

func (l Label) ID() database.ID {
	return l.Uuid
}
