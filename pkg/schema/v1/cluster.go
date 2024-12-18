package v1

import (
	"database/sql"
	"github.com/icinga/icinga-go-library/types"
)

type Cluster struct {
	Uuid types.UUID
	Name sql.NullString
}
