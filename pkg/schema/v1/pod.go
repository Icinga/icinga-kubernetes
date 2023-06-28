package v1

import (
	"database/sql"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/strcase"
	"github.com/icinga/icinga-kubernetes/pkg/types"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"strings"
)

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
	Conditions        []PodCondition `db:"-"`
	Containers        []Container    `db:"-"`
	Owners            []PodOwner     `db:"-"`
	Labels            []Label        `db:"-"`
	PodLabels         []PodLabel     `db:"-"`
	Pvcs              []PodPvc       `db:"-"`
	Volumes           []PodVolume    `db:"-"`
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
	for _, k8sContainer := range pod.Spec.Containers {
		var started bool
		if containerStatuses[k8sContainer.Name].Started != nil {
			started = *containerStatuses[k8sContainer.Name].Started
		}
		state, stateDetails, err := MarshalFirstNonNilStructFieldToJSON(containerStatuses[k8sContainer.Name].State)
		if err != nil {
			panic(err)
		}
		var containerState sql.NullString
		if state != "" {
			containerState.String = strcase.Snake(state)
			containerState.Valid = true
		}

		container := Container{
			ContainerMeta: ContainerMeta{
				Id:    types.Checksum(pod.Namespace + "/" + pod.Name + "/" + k8sContainer.Name),
				PodId: p.Id,
			},
			Name:           k8sContainer.Name,
			Image:          k8sContainer.Image,
			CpuLimits:      k8sContainer.Resources.Limits.Cpu().MilliValue(),
			CpuRequests:    k8sContainer.Resources.Requests.Cpu().MilliValue(),
			MemoryLimits:   k8sContainer.Resources.Limits.Memory().MilliValue(),
			MemoryRequests: k8sContainer.Resources.Requests.Memory().MilliValue(),
			Ready: types.Bool{
				Bool:  containerStatuses[k8sContainer.Name].Ready,
				Valid: true,
			},
			Started: types.Bool{
				Bool:  started,
				Valid: true,
			},
			RestartCount: containerStatuses[k8sContainer.Name].RestartCount,
			State:        containerState,
			StateDetails: stateDetails,
		}

		p.CpuLimits += k8sContainer.Resources.Limits.Cpu().MilliValue()
		p.CpuRequests += k8sContainer.Resources.Requests.Cpu().MilliValue()
		p.MemoryLimits += k8sContainer.Resources.Limits.Memory().MilliValue()
		p.MemoryRequests += k8sContainer.Resources.Requests.Memory().MilliValue()

		for _, device := range k8sContainer.VolumeDevices {
			container.ContainerDevices = append(container.ContainerDevices, ContainerDevice{
				ContainerId: container.Id,
				PodId:       p.Id,
				Name:        device.Name,
				Path:        device.DevicePath,
			})
		}

		for _, mount := range k8sContainer.VolumeMounts {
			var subPath sql.NullString
			if mount.SubPath != "" {
				subPath.String = mount.SubPath
				subPath.Valid = true
			}
			container.ContainerMounts = append(container.ContainerMounts, ContainerMount{
				ContainerId: container.Id,
				PodId:       p.Id,
				VolumeName:  mount.Name,
				Path:        mount.MountPath,
				SubPath:     subPath,
				ReadOnly: types.Bool{
					Bool:  mount.ReadOnly,
					Valid: true,
				},
			})
		}

		p.Containers = append(p.Containers, container)
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

func (p *Pod) Relations() []database.Relation {
	fk := database.WithForeignKey("pod_id")

	return []database.Relation{
		database.HasMany(p.Conditions, fk),
		database.HasMany(p.Containers, fk),
		database.HasMany(p.Owners, fk),
		database.HasMany(p.Labels, database.WithoutCascadeDelete()),
		database.HasMany(p.PodLabels, fk),
		database.HasMany(p.Pvcs, fk),
		database.HasMany(p.Volumes, fk),
	}
}
