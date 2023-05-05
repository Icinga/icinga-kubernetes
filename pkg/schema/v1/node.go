package v1

import corev1 "k8s.io/api/core/v1"

type Node struct {
	Name      string
	Namespace string
}

func NewNodeFromK8s(obj *corev1.Node) (*Node, error) {
	return &Node{
		Name:      obj.Name,
		Namespace: obj.Namespace,
	}, nil
}
