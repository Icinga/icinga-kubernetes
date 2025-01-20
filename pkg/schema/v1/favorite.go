package v1

import "github.com/icinga/icinga-go-library/types"

type Favorite struct {
	ResourceUuid types.UUID
	Kind         string
	Username     string
}
