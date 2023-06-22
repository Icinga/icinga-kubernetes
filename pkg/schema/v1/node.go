package v1

import (
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/types"
	"github.com/pkg/errors"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knet "k8s.io/utils/net"
	"net"
)

type Node struct {
	Meta
	Id                types.Binary
	PodCIDR           string
	NumIps            int64
	Unschedulable     types.Bool
	Ready             types.Bool
	CpuCapacity       int64
	CpuAllocatable    int64
	MemoryCapacity    int64
	MemoryAllocatable int64
	PodCapacity       int64
	Conditions        []NodeCondition `db:"-"`
	Volumes           []NodeVolume    `db:"-"`
}

type NodeCondition struct {
	NodeId         types.Binary
	Type           string
	Status         string
	LastHeartbeat  types.UnixMilli
	LastTransition types.UnixMilli
	Reason         string
	Message        string
}

type NodeVolume struct {
	NodeId     types.Binary
	name       kcorev1.UniqueVolumeName
	DevicePath string
	Mounted    types.Bool
}

func NewNode() Resource {
	return &Node{}
}

func (n *Node) Obtain(k8s kmetav1.Object) {
	n.ObtainMeta(k8s)

	node := k8s.(*kcorev1.Node)

	n.Id = types.Checksum(n.Namespace + "/" + n.Name)
	n.PodCIDR = node.Spec.PodCIDR
	_, cidr, err := net.ParseCIDR(n.PodCIDR)
	if err != nil {
		panic(errors.Wrapf(err, "failed to parse CIDR %s", n.PodCIDR))
	}
	n.NumIps = knet.RangeSize(cidr) - 2
	n.Unschedulable = types.Bool{
		Bool:  node.Spec.Unschedulable,
		Valid: true,
	}
	n.Ready = types.Bool{
		Bool:  getNodeConditionStatus(node, kcorev1.NodeReady),
		Valid: true,
	}
	n.CpuCapacity = node.Status.Capacity.Cpu().MilliValue()
	n.CpuAllocatable = node.Status.Allocatable.Cpu().MilliValue()
	n.MemoryCapacity = node.Status.Capacity.Memory().MilliValue()
	n.MemoryAllocatable = node.Status.Allocatable.Memory().MilliValue()
	n.PodCapacity = node.Status.Allocatable.Pods().Value()

	for _, condition := range node.Status.Conditions {
		n.Conditions = append(n.Conditions, NodeCondition{
			NodeId:         n.Id,
			Type:           string(condition.Type),
			Status:         string(condition.Status),
			LastHeartbeat:  types.UnixMilli(condition.LastHeartbeatTime.Time),
			LastTransition: types.UnixMilli(condition.LastTransitionTime.Time),
			Reason:         condition.Reason,
			Message:        condition.Message,
		})
	}

	volumesMounted := make(map[kcorev1.UniqueVolumeName]interface{}, len(node.Status.VolumesInUse))
	for _, name := range node.Status.VolumesInUse {
		volumesMounted[name] = struct{}{}
	}
	for _, volume := range node.Status.VolumesAttached {
		_, mounted := volumesMounted[volume.Name]
		n.Volumes = append(n.Volumes, NodeVolume{
			NodeId:     n.Id,
			name:       volume.Name,
			DevicePath: volume.DevicePath,
			Mounted: types.Bool{
				Bool:  mounted,
				Valid: true,
			},
		})
	}
}

func (n *Node) Relations() []database.Relation {
	fk := database.WithForeignKey("node_id")

	return []database.Relation{
		database.HasMany(n.Conditions, fk),
		database.HasMany(n.Volumes, fk),
	}
}

func getNodeConditionStatus(node *kcorev1.Node, conditionType kcorev1.NodeConditionType) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == conditionType {
			return true
		}
	}

	return false
}
