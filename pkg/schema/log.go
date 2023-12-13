package schema

import "github.com/icinga/icinga-go-library/types"

type ContainerLog struct {
	kmetaWithoutNamespace
	ContainerId types.Binary
	PodId       types.Binary
	Time        string
	Log         string
}
