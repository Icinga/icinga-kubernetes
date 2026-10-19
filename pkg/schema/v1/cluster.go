package v1

import (
	"database/sql"

	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	kdatabase "github.com/icinga/icinga-kubernetes/pkg/database"
)

type Cluster struct {
	kdatabase.EntityStub
	Uuid types.UUID
	Name sql.NullString
}

func (c Cluster) ID() database.ID {
	return c.Uuid
}
