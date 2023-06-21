package v1

import (
	"context"
	"fmt"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/strcase"
	"github.com/icinga/icinga-kubernetes/pkg/types"
	"io"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"strings"
)

type Container struct {
	Id             types.Binary
	PodId          types.Binary
	Name           string
	Image          string
	CpuLimits      int64
	CpuRequests    int64
	MemoryLimits   int64
	MemoryRequests int64
	State          string
	StateDetails   string
	Ready          types.Bool
	Started        types.Bool
	RestartCount   int32
	Logs           string
}

type ContainerDevice struct {
	ContainerId types.Binary
	PodId       types.Binary
	Name        string
	Path        string
}

type ContainerMount struct {
	ContainerId types.Binary
	PodId       types.Binary
	VolumeName  string
	Path        string
	SubPath     string
	ReadOnly    types.Bool
}

type PodFactory struct {
	clientset *kubernetes.Clientset
}

type Pod struct {
	Meta
	Id                types.Binary
	NodeName          string
	NominatedNodeName string
	Ip                string
	Phase             string
	CpuLimits         int64
	CpuRequests       int64
	MemoryLimits      int64
	MemoryRequests    int64
	Reason            string
	Message           string
	Qos               string
	RestartPolicy     string
	Conditions        []PodCondition    `db:"-"`
	Containers        []Container       `db:"-"`
	ContainerDevices  []ContainerDevice `db:"-"`
	ContainerMounts   []ContainerMount  `db:"-"`
	Owners            []PodOwner        `db:"-"`
	Labels            []Label           `db:"-"`
	PodLabels         []PodLabel        `db:"-"`
	Pvcs              []PodPvc          `db:"-"`
	Volumes           []PodVolume       `db:"-"`
	factory           *PodFactory
}

type PodCondition struct {
	PodId          types.Binary
	Type           string
	Status         string
	LastProbe      types.UnixMilli
	LastTransition types.UnixMilli
	Reason         string
	Message        string
}

type PodLabel struct {
	PodId   types.Binary
	LabelId types.Binary
}

type PodOwner struct {
	PodId              types.Binary
	Kind               string
	Name               string
	Uid                ktypes.UID
	Controller         types.Bool
	BlockOwnerDeletion types.Bool
}

type PodVolume struct {
	PodId      types.Binary
	VolumeName string
	Type       string
	Source     string
}

type PodPvc struct {
	PodId      types.Binary
	VolumeName string
	ClaimName  string
	ReadOnly   types.Bool
}

func NewPodFactory(clientset *kubernetes.Clientset) *PodFactory {
	return &PodFactory{
		clientset: clientset,
	}
}

func (f *PodFactory) New() Resource {
	return &Pod{factory: f}
}

func (p *Pod) Obtain(k8s kmetav1.Object) {
	p.ObtainMeta(k8s)

	pod := k8s.(*kcorev1.Pod)

	p.Id = types.Checksum(pod.Namespace + "/" + pod.Name)
	p.NodeName = pod.Spec.NodeName
	p.NominatedNodeName = pod.Status.NominatedNodeName
	p.Ip = pod.Status.PodIP
	p.Phase = strcase.Snake(string(pod.Status.Phase))
	p.Reason = pod.Status.Reason
	p.Message = pod.Status.Message
	p.Qos = strcase.Snake(string(pod.Status.QOSClass))
	p.RestartPolicy = strcase.Snake(string(pod.Spec.RestartPolicy))

	for _, condition := range pod.Status.Conditions {
		p.Conditions = append(p.Conditions, PodCondition{
			PodId:          p.Id,
			Type:           string(condition.Type),
			Status:         string(condition.Status),
			LastProbe:      types.UnixMilli(condition.LastProbeTime.Time),
			LastTransition: types.UnixMilli(condition.LastTransitionTime.Time),
			Reason:         condition.Reason,
			Message:        condition.Message,
		})
	}

	containerStatuses := make(map[string]kcorev1.ContainerStatus, len(pod.Spec.Containers))
	for _, containerStatus := range pod.Status.ContainerStatuses {
		containerStatuses[containerStatus.Name] = containerStatus
	}
	for _, container := range pod.Spec.Containers {
		if containerStatuses[container.Name].RestartCount > 0 {
			fmt.Println(containerStatuses[container.Name].LastTerminationState)
		}
		var started bool
		if containerStatuses[container.Name].Started != nil {
			started = *containerStatuses[container.Name].Started
		}
		state, stateDetails, err := MarshalFirstNonNilStructFieldToJSON(containerStatuses[container.Name].State)
		if err != nil {
			panic(err)
		}
		logs, err := getContainerLogs(p.factory.clientset, pod, container)
		if err != nil {
			// ContainerCreating, NotFound, ...
			fmt.Println(err)
			logs = ""
		}
		p.Containers = append(p.Containers, Container{
			Id:             types.Checksum(pod.Namespace + "/" + pod.Name + "/" + container.Name),
			PodId:          p.Id,
			Name:           container.Name,
			Image:          container.Image,
			CpuLimits:      container.Resources.Limits.Cpu().MilliValue(),
			CpuRequests:    container.Resources.Requests.Cpu().MilliValue(),
			MemoryLimits:   container.Resources.Limits.Memory().MilliValue(),
			MemoryRequests: container.Resources.Requests.Memory().MilliValue(),
			Ready: types.Bool{
				Bool:  containerStatuses[container.Name].Ready,
				Valid: true,
			},
			Started: types.Bool{
				Bool:  started,
				Valid: true,
			},
			RestartCount: containerStatuses[container.Name].RestartCount,
			State:        strcase.Snake(state),
			StateDetails: stateDetails,
			Logs:         logs,
		})

		p.CpuLimits += container.Resources.Limits.Cpu().MilliValue()
		p.CpuRequests += container.Resources.Requests.Cpu().MilliValue()
		p.MemoryLimits += container.Resources.Limits.Memory().MilliValue()
		p.MemoryRequests += container.Resources.Requests.Memory().MilliValue()

		for _, device := range container.VolumeDevices {
			p.ContainerDevices = append(p.ContainerDevices, ContainerDevice{
				ContainerId: types.Checksum(pod.Namespace + "/" + pod.Name + "/" + container.Name),
				PodId:       p.Id,
				Name:        device.Name,
				Path:        device.DevicePath,
			})
		}

		for _, mount := range container.VolumeMounts {
			p.ContainerMounts = append(p.ContainerMounts, ContainerMount{
				ContainerId: types.Checksum(pod.Namespace + "/" + pod.Name + "/" + container.Name),
				PodId:       p.Id,
				VolumeName:  mount.Name,
				Path:        mount.MountPath,
				SubPath:     mount.SubPath,
				ReadOnly: types.Bool{
					Bool:  mount.ReadOnly,
					Valid: true,
				},
			})
		}
	}

	for labelName, labelValue := range pod.Labels {
		labelId := types.Checksum(strings.ToLower(labelName + ":" + labelValue))
		p.Labels = append(p.Labels, Label{
			Id:    labelId,
			Name:  labelName,
			Value: labelValue,
		})
		p.PodLabels = append(p.PodLabels, PodLabel{
			PodId:   p.Id,
			LabelId: labelId,
		})
	}

	for _, ownerReference := range pod.OwnerReferences {
		var blockOwnerDeletion, controller bool
		if ownerReference.BlockOwnerDeletion != nil {
			blockOwnerDeletion = *ownerReference.BlockOwnerDeletion
		}
		if ownerReference.Controller != nil {
			controller = *ownerReference.Controller
		}
		p.Owners = append(p.Owners, PodOwner{
			PodId: p.Id,
			Kind:  strcase.Snake(ownerReference.Kind),
			Name:  ownerReference.Name,
			Uid:   ownerReference.UID,
			BlockOwnerDeletion: types.Bool{
				Bool:  blockOwnerDeletion,
				Valid: true,
			},
			Controller: types.Bool{
				Bool:  controller,
				Valid: true,
			},
		})
	}

	// https://kubernetes.io/docs/concepts/workloads/pods/init-containers/#resources
	for _, container := range pod.Spec.InitContainers {
		// Init container must complete successfully before the next one starts,
		// so we don't have to sum their resources.
		p.CpuLimits = types.MaxInt(p.CpuLimits, container.Resources.Limits.Cpu().MilliValue())
		p.CpuRequests = types.MaxInt(p.CpuRequests, container.Resources.Requests.Cpu().MilliValue())
		p.MemoryLimits = types.MaxInt(p.MemoryLimits, container.Resources.Limits.Memory().MilliValue())
		p.MemoryRequests = types.MaxInt(p.MemoryRequests, container.Resources.Requests.Memory().MilliValue())
	}

	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			p.Pvcs = append(p.Pvcs, PodPvc{
				PodId:      p.Id,
				VolumeName: volume.Name,
				ClaimName:  volume.PersistentVolumeClaim.ClaimName,
				ReadOnly: types.Bool{
					Bool:  volume.PersistentVolumeClaim.ReadOnly,
					Valid: true,
				},
			})
		} else {
			t, source, err := MarshalFirstNonNilStructFieldToJSON(volume.VolumeSource)
			if err != nil {
				panic(err)
			}

			p.Volumes = append(p.Volumes, PodVolume{
				PodId:      p.Id,
				VolumeName: volume.Name,
				Type:       t,
				Source:     source,
			})
		}
	}
}

func (p *Pod) Relations() database.Relations {
	return database.Relations{
		database.HasMany[PodCondition]{
			Entities:    p.Conditions,
			ForeignKey_: "pod_id",
		},
		database.HasMany[Container]{
			Entities:    p.Containers,
			ForeignKey_: "pod_id",
		},
		database.HasMany[ContainerDevice]{
			Entities:    p.ContainerDevices,
			ForeignKey_: "pod_id",
		},
		database.HasMany[ContainerMount]{
			Entities:    p.ContainerMounts,
			ForeignKey_: "pod_id",
		},
		database.HasMany[PodOwner]{
			Entities:    p.Owners,
			ForeignKey_: "pod_id",
		},
		database.HasMany[Label]{
			Entities:    p.Labels,
			ForeignKey_: "value", // TODO: This is a hack to not delete any labels.
		},
		database.HasMany[PodLabel]{
			Entities:    p.PodLabels,
			ForeignKey_: "pod_id",
		},
		database.HasMany[PodPvc]{
			Entities:    p.Pvcs,
			ForeignKey_: "pod_id",
		},
		database.HasMany[PodVolume]{
			Entities:    p.Volumes,
			ForeignKey_: "pod_id",
		},
	}
}

func getContainerLogs(clientset *kubernetes.Clientset, pod *kcorev1.Pod, container kcorev1.Container) (string, error) {
	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &kcorev1.PodLogOptions{Container: container.Name})
	body, err := req.Stream(context.TODO())
	if err != nil {
		return "", err
	}
	defer body.Close()
	logs, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}

	return string(logs), nil
}
