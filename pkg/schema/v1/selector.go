package v1

import (
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/kdatabase"
)

type Selector struct {
	kdatabase.EntityStub
	Uuid  types.UUID
	Name  string
	Value string
}

func (s Selector) ID() database.ID {
	return s.Uuid
}
