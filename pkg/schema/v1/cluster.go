package v1

import (
	"database/sql"

	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/kdatabase"
)

type Cluster struct {
	kdatabase.EntityStub
	Uuid types.UUID
	Name sql.NullString
}

func (c Cluster) ID() database.ID {
	return c.Uuid
}
