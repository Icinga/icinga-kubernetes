package v1

import (
	"github.com/icinga/icinga-kubernetes/pkg/contracts"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/types"
	"github.com/pkg/errors"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knet "k8s.io/utils/net"
	"net"
)

type Node struct {
	ResourceMeta
	PodCIDR           string
	NumIps            int64
	Unschedulable     types.Bool
	Ready             types.Bool
	CpuCapacity       int64
	CpuAllocatable    int64
	MemoryCapacity    int64
	MemoryAllocatable int64
	PodCapacity       int64
	Conditions        []*NodeCondition `db:"-" hash:"-"`
	Volumes           []*NodeVolume    `db:"-" hash:"-"`
}

type NodeMeta struct {
	contracts.Meta
	NodeId types.Binary
}

func (nm *NodeMeta) Fingerprint() contracts.FingerPrinter {
	return nm
}

func (nm *NodeMeta) ParentID() types.Binary {
	return nm.NodeId
}

type NodeCondition struct {
	NodeMeta
	Type           string
	Status         string
	LastHeartbeat  types.UnixMilli
	LastTransition types.UnixMilli
	Reason         string
	Message        string
}

type NodeVolume struct {
	NodeMeta
	name       kcorev1.UniqueVolumeName
	DevicePath string
	Mounted    types.Bool
}

func NewNode() contracts.Entity {
	return &Node{}
}

func (n *Node) Obtain(k8s kmetav1.Object) {
	n.ObtainMeta(k8s)

	node := k8s.(*kcorev1.Node)

	n.Id = types.Checksum(n.Namespace + "/" + n.Name)
	n.PodCIDR = node.Spec.PodCIDR
	_, cidr, err := net.ParseCIDR(n.PodCIDR)
	// TODO(yh): Make field NumIps nullable and don't panic here!
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

	n.PropertiesChecksum = types.HashStruct(n)

	for _, condition := range node.Status.Conditions {
		nodeCond := &NodeCondition{
			NodeMeta: NodeMeta{
				NodeId: n.Id,
				Meta:   contracts.Meta{Id: types.Checksum(types.MustPackSlice(n.Id, condition.Type))},
			},
			Type:           string(condition.Type),
			Status:         string(condition.Status),
			LastHeartbeat:  types.UnixMilli(condition.LastHeartbeatTime.Time),
			LastTransition: types.UnixMilli(condition.LastTransitionTime.Time),
			Reason:         condition.Reason,
			Message:        condition.Message,
		}
		nodeCond.PropertiesChecksum = types.HashStruct(nodeCond)

		n.Conditions = append(n.Conditions, nodeCond)
	}

	volumesMounted := make(map[kcorev1.UniqueVolumeName]interface{}, len(node.Status.VolumesInUse))
	for _, name := range node.Status.VolumesInUse {
		volumesMounted[name] = struct{}{}
	}
	for _, volume := range node.Status.VolumesAttached {
		_, mounted := volumesMounted[volume.Name]
		nodeVolume := &NodeVolume{
			NodeMeta: NodeMeta{
				NodeId: n.Id,
				Meta:   contracts.Meta{Id: types.Checksum(types.MustPackSlice(n.Id, volume.Name))},
			},
			name:       volume.Name,
			DevicePath: volume.DevicePath,
			Mounted: types.Bool{
				Bool:  mounted,
				Valid: true,
			},
		}
		nodeVolume.PropertiesChecksum = types.HashStruct(nodeVolume)

		n.Volumes = append(n.Volumes, nodeVolume)
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
