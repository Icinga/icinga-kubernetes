package schema

import (
	"github.com/icinga/icinga-kubernetes/pkg/contracts"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Pod struct {
	kmetaWithNamespace
}

func NewPod() contracts.Resource {
	return &Pod{}
}

func (p *Pod) Obtain(kobject kmetav1.Object) {
	p.kmetaWithNamespace.Obtain(kobject)
}
