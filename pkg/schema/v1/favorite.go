package v1

import (
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	kdatabase "github.com/icinga/icinga-kubernetes/pkg/database"
)

type Favorite struct {
	kdatabase.EntityStub
	ResourceUuid types.UUID
	Kind         string
	Username     string
	Priority     int
}

func (f Favorite) ID() database.ID {
	return compoundId{id: f.ResourceUuid.String() + f.Username}
}
