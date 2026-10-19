package v1

import (
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/kdatabase"
)

type ResourceLabel struct {
	kdatabase.EntityStub
	ResourceUuid types.UUID
	LabelUuid    types.UUID
}

func (r ResourceLabel) ID() database.ID {
	return compoundId{id: r.ResourceUuid.String() + r.LabelUuid.String()}
}
