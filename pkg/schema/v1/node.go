package v1

import (
	"github.com/icinga/icinga-kubernetes/pkg/types"
	corev1 "k8s.io/api/core/v1"
)

type Node struct {
	Name          string
	Namespace     string
	PodCIDR       string `db:"pod_cidr"`
	Unschedulable types.Bool
	Ready         types.Bool
	Created       types.UnixMilli
}

func NewNodeFromK8s(obj *corev1.Node) (*Node, error) {
	return &Node{
		Name:      obj.Name,
		Namespace: obj.Namespace,
		PodCIDR:   obj.Spec.PodCIDR,
		Unschedulable: types.Bool{
			Bool:  obj.Spec.Unschedulable,
			Valid: true,
		},
		Ready: types.Bool{
			Bool:  getNodeConditionStatus(obj, corev1.NodeReady),
			Valid: true,
		},
		Created: types.UnixMilli(obj.CreationTimestamp.Time),
	}, nil
}

func getNodeConditionStatus(obj *corev1.Node, conditionType corev1.NodeConditionType) bool {
	for _, condition := range obj.Status.Conditions {
		if condition.Type == conditionType {
			return true
		}
	}
	return false
}
