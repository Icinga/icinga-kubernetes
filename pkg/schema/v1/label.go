package v1

import (
	"github.com/icinga/icinga-go-library/types"
)

type Label struct {
	Uuid  types.UUID
	Name  string
	Value string
}