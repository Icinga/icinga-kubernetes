package schema

import (
	"github.com/icinga/icinga-kubernetes/pkg/contracts"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Node struct {
	kmetaWithoutNamespace
}

func NewNode() contracts.Resource {
	return &Node{}
}

func (n *Node) Obtain(kobject kmetav1.Object) {
	n.kmetaWithoutNamespace.Obtain(kobject)
}
