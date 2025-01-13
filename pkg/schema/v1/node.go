package v1

import (
	"fmt"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/notifications"
	"github.com/pkg/errors"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	kserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	knet "k8s.io/utils/net"
	"net"
	"net/url"
	"strings"
)

type Node struct {
	Meta
	PodCIDR                 string
	NumIps                  int64
	Unschedulable           types.Bool
	Ready                   types.Bool
	CpuCapacity             int64
	CpuAllocatable          int64
	MemoryCapacity          int64
	MemoryAllocatable       int64
	PodCapacity             int64
	Yaml                    string
	Roles                   string
	MachineId               string
	SystemUUID              string
	BootId                  string
	KernelVersion           string
	OsImage                 string
	OperatingSystem         string
	Architecture            string
	ContainerRuntimeVersion string
	KubeletVersion          string
	KubeProxyVersion        string
	IcingaState             IcingaState
	IcingaStateReason       string
	Conditions              []NodeCondition      `db:"-"`
	Volumes                 []NodeVolume         `db:"-"`
	Labels                  []Label              `db:"-"`
	NodeLabels              []NodeLabel          `db:"-"`
	ResourceLabels          []ResourceLabel      `db:"-"`
	Annotations             []Annotation         `db:"-"`
	NodeAnnotations         []NodeAnnotation     `db:"-"`
	ResourceAnnotations     []ResourceAnnotation `db:"-"`
}

type NodeCondition struct {
	NodeUuid       types.UUID
	Type           string
	Status         string
	LastHeartbeat  types.UnixMilli
	LastTransition types.UnixMilli
	Reason         string
	Message        string
}

type NodeVolume struct {
	NodeUuid   types.UUID
	Name       kcorev1.UniqueVolumeName
	DevicePath string
	Mounted    types.Bool
}

type NodeLabel struct {
	NodeUuid  types.UUID
	LabelUuid types.UUID
}

type NodeAnnotation struct {
	NodeUuid       types.UUID
	AnnotationUuid types.UUID
}

func NewNode() Resource {
	return &Node{}
}

func (n *Node) Obtain(k8s kmetav1.Object, clusterUuid types.UUID) {
	n.ObtainMeta(k8s, clusterUuid)

	node := k8s.(*kcorev1.Node)

	n.PodCIDR = node.Spec.PodCIDR
	if n.PodCIDR != "" {
		_, cidr, err := net.ParseCIDR(n.PodCIDR)
		if err != nil {
			panic(errors.Wrapf(err, "failed to parse CIDR %s", n.PodCIDR))
		}
		n.NumIps = knet.RangeSize(cidr) - 2
	}
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
	n.MachineId = node.Status.NodeInfo.MachineID
	n.SystemUUID = node.Status.NodeInfo.SystemUUID
	n.BootId = node.Status.NodeInfo.BootID
	n.KernelVersion = node.Status.NodeInfo.KernelVersion
	n.OsImage = node.Status.NodeInfo.OSImage
	n.OperatingSystem = node.Status.NodeInfo.OperatingSystem
	n.Architecture = node.Status.NodeInfo.Architecture
	n.ContainerRuntimeVersion = node.Status.NodeInfo.ContainerRuntimeVersion
	n.KubeletVersion = node.Status.NodeInfo.KubeletVersion
	n.KubeProxyVersion = node.Status.NodeInfo.KubeProxyVersion

	var roles []string
	for labelName := range node.Labels {
		if strings.Contains(labelName, "node-role") {
			role := strings.SplitAfter(labelName, "/")[1]
			roles = append(roles, role)
		}
	}
	n.Roles = strings.Join(roles, ", ")

	n.IcingaState, n.IcingaStateReason = n.getIcingaState(node)

	for _, condition := range node.Status.Conditions {
		n.Conditions = append(n.Conditions, NodeCondition{
			NodeUuid:       n.Uuid,
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
			NodeUuid:   n.Uuid,
			Name:       volume.Name,
			DevicePath: volume.DevicePath,
			Mounted: types.Bool{
				Bool:  mounted,
				Valid: true,
			},
		})
	}

	for labelName, labelValue := range node.Labels {
		labelUuid := NewUUID(n.Uuid, strings.ToLower(labelName+":"+labelValue))
		n.Labels = append(n.Labels, Label{
			Uuid:  labelUuid,
			Name:  labelName,
			Value: labelValue,
		})
		n.NodeLabels = append(n.NodeLabels, NodeLabel{
			NodeUuid:  n.Uuid,
			LabelUuid: labelUuid,
		})
		n.ResourceLabels = append(n.ResourceLabels, ResourceLabel{
			ResourceUuid: n.Uuid,
			LabelUuid:    labelUuid,
		})
	}

	scheme := kruntime.NewScheme()
	_ = kcorev1.AddToScheme(scheme)
	codec := kserializer.NewCodecFactory(scheme).EncoderForVersion(kjson.NewYAMLSerializer(kjson.DefaultMetaFactory, scheme, scheme), kcorev1.SchemeGroupVersion)
	output, _ := kruntime.Encode(codec, node)
	n.Yaml = string(output)

	for annotationName, annotationValue := range node.Annotations {
		annotationUuid := NewUUID(n.Uuid, strings.ToLower(annotationName+":"+annotationValue))
		n.Annotations = append(n.Annotations, Annotation{
			Uuid:  annotationUuid,
			Name:  annotationName,
			Value: annotationValue,
		})
		n.NodeAnnotations = append(n.NodeAnnotations, NodeAnnotation{
			NodeUuid:       n.Uuid,
			AnnotationUuid: annotationUuid,
		})
		n.ResourceAnnotations = append(n.ResourceAnnotations, ResourceAnnotation{
			ResourceUuid:   n.Uuid,
			AnnotationUuid: annotationUuid,
		})
	}
}

func (n *Node) MarshalEvent() (notifications.Event, error) {
	return notifications.Event{
		Name:     n.Namespace + "/" + n.Name,
		Severity: n.IcingaState.ToSeverity(),
		Message:  n.IcingaStateReason,
		URL:      &url.URL{Path: "/node", RawQuery: fmt.Sprintf("id=%s", n.Uuid)},
		Tags: map[string]string{
			"uuid":      n.Uuid.String(),
			"name":      n.Name,
			"namespace": n.Namespace,
			"resource":  "node",
		},
	}, nil
}

func (n *Node) getIcingaState(node *kcorev1.Node) (IcingaState, string) {
	// if node.Status.Phase == kcorev1.NodePending {
	//	return Pending, fmt.Sprintf("Node %s is pending.", node.Name)
	// }
	//
	// if node.Status.Phase == kcorev1.NodeTerminated {
	//	return Ok, fmt.Sprintf("Node %s is terminated.", node.Name)
	// }

	var state IcingaState
	var reason []string

	for _, condition := range node.Status.Conditions {
		if condition.Status == kcorev1.ConditionTrue {
			switch condition.Type {
			case kcorev1.NodeDiskPressure:
				state = Critical
				reason = append(reason, fmt.Sprintf("Node %s is running out of disk space", node.Name))
			case kcorev1.NodeMemoryPressure:
				state = Critical
				reason = append(reason, fmt.Sprintf("Node %s is running out of available memory", node.Name))
			case kcorev1.NodePIDPressure:
				state = Critical
				reason = append(reason, fmt.Sprintf("Node %s is running out of process IDs", node.Name))
			case kcorev1.NodeNetworkUnavailable:
				state = Critical
				reason = append(reason, fmt.Sprintf("Node %s network is not correctly configured", node.Name))
			}
		}

		if condition.Status == kcorev1.ConditionFalse && condition.Type == kcorev1.NodeReady {
			state = Critical
			reason = append(reason, fmt.Sprintf("Node %s is not ready", node.Name))
		}
	}

	if state != Ok {
		return state, strings.Join(reason, ". ") + "."
	}

	return Ok, fmt.Sprintf("Node %s is ok.", node.Name)
}

func (n *Node) Relations() []database.Relation {
	fk := database.WithForeignKey("node_uuid")

	return []database.Relation{
		database.HasMany(n.Conditions, fk),
		database.HasMany(n.Volumes, fk),
		database.HasMany(n.ResourceLabels, database.WithForeignKey("resource_uuid")),
		database.HasMany(n.Labels, database.WithoutCascadeDelete()),
		database.HasMany(n.NodeLabels, fk),
		database.HasMany(n.ResourceAnnotations, database.WithForeignKey("resource_uuid")),
		database.HasMany(n.Annotations, database.WithoutCascadeDelete()),
		database.HasMany(n.NodeAnnotations, fk),
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
