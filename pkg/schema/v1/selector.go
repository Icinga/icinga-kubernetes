package v1

import "github.com/icinga/icinga-kubernetes/pkg/types"

type Selector struct {
	Id    types.Binary
	Name  string
	Value string
}
