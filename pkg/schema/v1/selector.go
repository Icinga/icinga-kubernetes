package v1

import "github.com/icinga/icinga-go-library/types"

type Selector struct {
	Id    types.Binary
	Name  string
	Value string
}
