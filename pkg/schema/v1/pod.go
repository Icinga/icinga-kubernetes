package v1

import (
	"database/sql"
	"github.com/icinga/icinga-kubernetes/pkg/contracts"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/strcase"
	"github.com/icinga/icinga-kubernetes/pkg/types"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

type PodFactory struct {
	clientset *kubernetes.Clientset
}

type Pod struct {
	ResourceMeta
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
	Conditions        []*PodCondition `db:"-"`
	Containers        []*Container    `db:"-"`
	Owners            []*PodOwner     `db:"-"`
	Labels            []*Label        `db:"-"`
	Pvcs              []*PodPvc       `db:"-"`
	Volumes           []*PodVolume    `db:"-"`
	factory           *PodFactory
}

type PodMeta struct {
	contracts.Meta
	PodId types.Binary `db:"pod_id"`
}

func (pm *PodMeta) Fingerprint() contracts.FingerPrinter {
	return pm
}

func (pm *PodMeta) ParentID() types.Binary {
	return pm.PodId
}

type PodCondition struct {
	PodMeta
	Type           string
	Status         string
	LastProbe      types.UnixMilli
	LastTransition types.UnixMilli
	Reason         string
	Message        string
}

type PodOwner struct {
	PodMeta
	Kind               string
	Name               string
	Uid                ktypes.UID
	Controller         types.Bool
	BlockOwnerDeletion types.Bool
}

type PodVolume struct {
	PodMeta
	VolumeName string
	Type       string
	Source     string
}

type PodPvc struct {
	PodMeta
	VolumeName string
	ClaimName  string
	ReadOnly   types.Bool
}

func NewPodFactory(clientset *kubernetes.Clientset) *PodFactory {
	return &PodFactory{
		clientset: clientset,
	}
}

func (f *PodFactory) New() contracts.Entity {
	return &Pod{factory: f}
}

func (p *Pod) Obtain(k8s kmetav1.Object) {
	p.ObtainMeta(k8s)
	defer func() {
		p.PropertiesChecksum = types.Checksum(MustMarshalJSON(p))
	}()

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
		podCond := &PodCondition{
			PodMeta: PodMeta{
				PodId: p.Id,
				Meta:  contracts.Meta{Id: types.Checksum(types.MustPackSlice(p.Id, condition.Type))},
			},
			Type:           string(condition.Type),
			Status:         string(condition.Status),
			LastProbe:      types.UnixMilli(condition.LastProbeTime.Time),
			LastTransition: types.UnixMilli(condition.LastTransitionTime.Time),
			Reason:         condition.Reason,
			Message:        condition.Message,
		}
		podCond.PropertiesChecksum = types.Checksum(MustMarshalJSON(podCond))

		p.Conditions = append(p.Conditions, podCond)
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

		container := &Container{
			PodMeta: PodMeta{
				PodId: p.Id,
				Meta:  contracts.Meta{Id: types.Checksum(pod.Namespace + "/" + pod.Name + "/" + k8sContainer.Name)},
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
		container.PropertiesChecksum = types.Checksum(MustMarshalJSON(container))

		p.CpuLimits += k8sContainer.Resources.Limits.Cpu().MilliValue()
		p.CpuRequests += k8sContainer.Resources.Requests.Cpu().MilliValue()
		p.MemoryLimits += k8sContainer.Resources.Limits.Memory().MilliValue()
		p.MemoryRequests += k8sContainer.Resources.Requests.Memory().MilliValue()

		for _, device := range k8sContainer.VolumeDevices {
			cd := &ContainerDevice{
				ContainerMeta: ContainerMeta{
					Meta:        contracts.Meta{Id: types.Checksum(types.MustPackSlice(p.Id, container.Id, device.Name))},
					ContainerId: container.Id,
				},
				Name: device.Name,
				Path: device.DevicePath,
			}
			cd.PropertiesChecksum = types.Checksum(MustMarshalJSON(cd))

			container.Devices = append(container.Devices, cd)
		}

		for _, mount := range k8sContainer.VolumeMounts {
			cm := &ContainerMount{
				ContainerMeta: ContainerMeta{
					Meta:        contracts.Meta{Id: types.Checksum(types.MustPackSlice(p.Id, container.Id, mount.Name))},
					ContainerId: container.Id,
				},
				VolumeName: mount.Name,
				Path:       mount.MountPath,
				SubPath:    mount.SubPath,
				ReadOnly: types.Bool{
					Bool:  mount.ReadOnly,
					Valid: true,
				},
			}
			cm.PropertiesChecksum = types.Checksum(MustMarshalJSON(cm))

			container.Mounts = append(container.Mounts, cm)
		}
	}

	for labelName, labelValue := range pod.Labels {
		label := NewLabel(labelName, labelValue)
		label.PodId = p.Id
		label.PropertiesChecksum = types.Checksum(MustMarshalJSON(label))

		p.Labels = append(p.Labels, label)
	}

	for _, ownerReference := range pod.OwnerReferences {
		var blockOwnerDeletion, controller bool
		if ownerReference.BlockOwnerDeletion != nil {
			blockOwnerDeletion = *ownerReference.BlockOwnerDeletion
		}
		if ownerReference.Controller != nil {
			controller = *ownerReference.Controller
		}
		owner := &PodOwner{
			PodMeta: PodMeta{
				PodId: p.Id,
				Meta:  contracts.Meta{Id: types.Checksum(types.MustPackSlice(p.Id, ownerReference.UID))},
			},
			Kind: strcase.Snake(ownerReference.Kind),
			Name: ownerReference.Name,
			Uid:  ownerReference.UID,
			BlockOwnerDeletion: types.Bool{
				Bool:  blockOwnerDeletion,
				Valid: true,
			},
			Controller: types.Bool{
				Bool:  controller,
				Valid: true,
			},
		}
		owner.PropertiesChecksum = types.Checksum(MustMarshalJSON(owner))

		p.Owners = append(p.Owners, owner)
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
			pvc := &PodPvc{
				PodMeta: PodMeta{
					PodId: p.Id,
					Meta:  contracts.Meta{Id: types.Checksum(types.MustPackSlice(p.Id, volume.Name, volume.PersistentVolumeClaim.ClaimName))},
				},
				VolumeName: volume.Name,
				ClaimName:  volume.PersistentVolumeClaim.ClaimName,
				ReadOnly: types.Bool{
					Bool:  volume.PersistentVolumeClaim.ReadOnly,
					Valid: true,
				},
			}
			pvc.PropertiesChecksum = types.Checksum(MustMarshalJSON(pvc))

			p.Pvcs = append(p.Pvcs, pvc)
		} else {
			t, source, err := MarshalFirstNonNilStructFieldToJSON(volume.VolumeSource)
			if err != nil {
				panic(err)
			}

			vol := &PodVolume{
				PodMeta: PodMeta{
					PodId: p.Id,
					Meta:  contracts.Meta{Id: types.Checksum(types.MustPackSlice(p.Id, volume.Name))},
				},
				VolumeName: volume.Name,
				Type:       t,
				Source:     source,
			}
			vol.PropertiesChecksum = types.Checksum(MustMarshalJSON(vol))

			p.Volumes = append(p.Volumes, vol)
		}
	}
}

func (p *Pod) Relations() []database.Relation {
	fk := database.WithForeignKey("pod_id")

	return []database.Relation{
		database.HasMany(p.Containers, fk, database.WithoutCascadeDelete()),
		database.HasMany(p.Owners, fk),
		database.HasMany(p.Labels, fk),
		database.HasMany(p.Pvcs, fk),
		database.HasMany(p.Volumes, fk),
	}
}

var (
	_ contracts.Entity   = (*Pod)(nil)
	_ contracts.Resource = (*Pod)(nil)
	_ contracts.Entity   = (*PodCondition)(nil)
)
