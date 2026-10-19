package v1

import (
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	kdatabase "github.com/icinga/icinga-kubernetes/pkg/database"
)

type Annotation struct {
	kdatabase.EntityStub
	Uuid  types.UUID
	Name  string
	Value string
}

func (a Annotation) ID() database.ID {
	return a.Uuid
}
