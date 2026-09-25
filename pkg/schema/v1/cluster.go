package v1

import (
	"database/sql"

	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
)

type Cluster struct {
	Uuid types.UUID
	Name sql.NullString
}

func (c Cluster) ID() database.ID {
	return c.Uuid
}

func (c Cluster) SetID(_ database.ID) {
	panic("Not expected to be called")
}

func (c Cluster) Fingerprint() database.Fingerprinter {
	return c
}
