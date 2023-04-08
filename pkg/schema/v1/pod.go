package v1

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

type Pod struct {
	// TODO: Add fields to be synchronized to the database.
	Name      string
	Namespace string
	UID       types.UID
	Phase     string
}

func NewPodFromK8s(obj *v1.Pod) (*Pod, error) {
	// TODO: Implement mapping from Kubernetes Pod objects.
	return &Pod{
		Name:      obj.Name,
		Namespace: obj.Namespace,
		UID:       obj.UID,
		Phase:     string(obj.Status.Phase),
	}, nil
}
