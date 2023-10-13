package schema

import (
	"github.com/icinga/icinga-kubernetes/pkg/contracts"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Namespace struct {
	kmetaWithoutNamespace
}

func NewNamespace() contracts.Resource {
	return &Namespace{}
}

func (n *Namespace) Obtain(kobject kmetav1.Object) {
	n.kmetaWithoutNamespace.Obtain(kobject)
}
