package v1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

type Pod struct {
	Name      string
	Namespace string
	UID       types.UID
	Phase     string
}

func NewPodFromK8s(obj *corev1.Pod) (*Pod, error) {
	return &Pod{
		Name:      obj.Name,
		Namespace: obj.Namespace,
		UID:       obj.UID,
		Phase:     string(obj.Status.Phase),
	}, nil
}
