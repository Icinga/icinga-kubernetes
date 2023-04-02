package v1

import v1 "k8s.io/api/core/v1"

type Pod struct {
	// TODO: Add fields to be synchronized to the database.
	Name string
}

func NewPodFromK8s(obj *v1.Pod) (*Pod, error) {
	// TODO: Implement mapping from Kubernetes Pod objects.
	return &Pod{
		Name: obj.Name,
	}, nil
}
