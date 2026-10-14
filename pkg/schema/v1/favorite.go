package v1

import (
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
)

type Favorite struct {
	ResourceUuid types.UUID
	Kind         string
	Username     string
	Priority     int
}

func (f Favorite) ID() database.ID {
	return compoundId{id: f.ResourceUuid.String() + f.Username}
}

func (f Favorite) SetID(_ database.ID) {
	panic("Not expected to be called")
}

func (f Favorite) Fingerprint() database.Fingerprinter {
	return f
}
