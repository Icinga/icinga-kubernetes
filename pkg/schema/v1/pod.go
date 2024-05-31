package v1

import (
	"database/sql"
	"fmt"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/strcase"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	kserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	ktypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"strings"
	"time"
)

const prolongedInitializationThreshold = 10 * time.Minute

type PodFactory struct {
	clientset *kubernetes.Clientset
}

type Pod struct {
	Meta
	NodeName          string
	NominatedNodeName string
	Ip                string
	Phase             string
	IcingaState       IcingaState
	IcingaStateReason string
	CpuLimits         int64
	CpuRequests       int64
	MemoryLimits      int64
	MemoryRequests    int64
	Reason            string
	Message           string
	Qos               string
	RestartPolicy     string
	Yaml              string
	Conditions        []PodCondition  `db:"-"`
	Containers        []Container     `db:"-"`
	Owners            []PodOwner      `db:"-"`
	Labels            []Label         `db:"-"`
	PodLabels         []PodLabel      `db:"-"`
	Annotations       []Annotation    `db:"-"`
	PodAnnotations    []PodAnnotation `db:"-"`
	Pvcs              []PodPvc        `db:"-"`
	Volumes           []PodVolume     `db:"-"`
	factory           *PodFactory
}

type PodYaml struct {
	PodId      types.Binary
	Kind       string
	ApiVersion string
	YamlData   string
}

type PodCondition struct {
	PodUuid        types.UUID
	Type           string
	Status         string
	LastProbe      types.UnixMilli
	LastTransition types.UnixMilli
	Reason         string
	Message        string
}

type PodLabel struct {
	PodUuid   types.UUID
	LabelUuid types.UUID
}

type PodAnnotation struct {
	PodUuid        types.UUID
	AnnotationUuid types.UUID
}

type PodOwner struct {
	PodUuid            types.UUID
	OwnerUuid          types.UUID
	Kind               string
	Name               string
	Uid                ktypes.UID
	Controller         types.Bool
	BlockOwnerDeletion types.Bool
}

type PodVolume struct {
	PodUuid    types.UUID
	VolumeName string
	Type       string
	Source     string
}

type PodPvc struct {
	PodUuid    types.UUID
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

	p.NodeName = pod.Spec.NodeName
	p.NominatedNodeName = pod.Status.NominatedNodeName
	p.Ip = pod.Status.PodIP
	p.Phase = strcase.Snake(string(pod.Status.Phase))
	p.IcingaState, p.IcingaStateReason = p.getIcingaState(pod)
	p.Reason = pod.Status.Reason
	p.Message = pod.Status.Message
	p.Qos = strcase.Snake(string(pod.Status.QOSClass))
	p.RestartPolicy = strcase.Snake(string(pod.Spec.RestartPolicy))

	for _, condition := range pod.Status.Conditions {
		p.Conditions = append(p.Conditions, PodCondition{
			PodUuid:        p.Uuid,
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
				Uuid:    NewUUID(p.Uuid, k8sContainer.Name),
				PodUuid: p.Uuid,
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
			container.Devices = append(container.Devices, ContainerDevice{
				ContainerUuid: container.Uuid,
				PodUuid:       p.Uuid,
				Name:          device.Name,
				Path:          device.DevicePath,
			})
		}

		for _, mount := range k8sContainer.VolumeMounts {
			var subPath sql.NullString
			if mount.SubPath != "" {
				subPath.String = mount.SubPath
				subPath.Valid = true
			}
			container.Mounts = append(container.Mounts, ContainerMount{
				ContainerUuid: container.Uuid,
				PodUuid:       p.Uuid,
				VolumeName:    mount.Name,
				Path:          mount.MountPath,
				SubPath:       subPath,
				ReadOnly: types.Bool{
					Bool:  mount.ReadOnly,
					Valid: true,
				},
			})
		}

		p.Containers = append(p.Containers, container)
	}

	for labelName, labelValue := range pod.Labels {
		labelUuid := NewUUID(p.Uuid, strings.ToLower(labelName+":"+labelValue))
		p.Labels = append(p.Labels, Label{
			Uuid:  labelUuid,
			Name:  labelName,
			Value: labelValue,
		})
		p.PodLabels = append(p.PodLabels, PodLabel{
			PodUuid:   p.Uuid,
			LabelUuid: labelUuid,
		})
	}

	for annotationName, annotationValue := range pod.Annotations {
		annotationUuid := NewUUID(p.Uuid, strings.ToLower(annotationName+":"+annotationValue))
		p.Annotations = append(p.Annotations, Annotation{
			Uuid:  annotationUuid,
			Name:  annotationName,
			Value: annotationValue,
		})
		p.PodAnnotations = append(p.PodAnnotations, PodAnnotation{
			PodUuid:        p.Uuid,
			AnnotationUuid: annotationUuid,
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
			PodUuid:   p.Uuid,
			OwnerUuid: EnsureUUID(p.Uid),
			Kind:      strcase.Snake(ownerReference.Kind),
			Name:      ownerReference.Name,
			Uid:       ownerReference.UID,
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
		p.CpuLimits = MaxInt(p.CpuLimits, container.Resources.Limits.Cpu().MilliValue())
		p.CpuRequests = MaxInt(p.CpuRequests, container.Resources.Requests.Cpu().MilliValue())
		p.MemoryLimits = MaxInt(p.MemoryLimits, container.Resources.Limits.Memory().MilliValue())
		p.MemoryRequests = MaxInt(p.MemoryRequests, container.Resources.Requests.Memory().MilliValue())
	}

	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			p.Pvcs = append(p.Pvcs, PodPvc{
				PodUuid:    p.Uuid,
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
				PodUuid:    p.Uuid,
				VolumeName: volume.Name,
				Type:       t,
				Source:     source,
			})
		}
	}

	scheme := kruntime.NewScheme()
	_ = kcorev1.AddToScheme(scheme)
	codec := kserializer.NewCodecFactory(scheme).EncoderForVersion(kjson.NewYAMLSerializer(kjson.DefaultMetaFactory, scheme, scheme), kcorev1.SchemeGroupVersion)
	output, _ := kruntime.Encode(codec, pod)
	p.Yaml = string(output)
}

func (p *Pod) getIcingaState(pod *kcorev1.Pod) (IcingaState, string) {
	readyContainers := 0
	state := Unknown
	reason := string(pod.Status.Phase)

	if pod.Status.Reason != "" {
		reason = fmt.Sprintf("Pod %s/%s is in %s state with reason: %s", pod.Namespace, pod.Name, pod.Status.Phase, pod.Status.Reason)
	}

	if pod.DeletionTimestamp != nil {
		reason = fmt.Sprintf("Pod %s/%s is being deleted", pod.Namespace, pod.Name)

		return Ok, reason
	}

	// If the Pod carries {type:PodScheduled, reason:SchedulingGated}, set reason to 'SchedulingGated'.
	for _, condition := range pod.Status.Conditions {
		if condition.Type == kcorev1.PodScheduled && condition.Reason == kcorev1.PodReasonSchedulingGated {
			state = Critical
			reason = fmt.Sprintf("Pod %s/%s scheduling skipped because one or more scheduling gates are still present: %s", pod.Namespace, pod.Name, kcorev1.PodReasonSchedulingGated)
		}
	}

	initializing := false
	for i, container := range pod.Status.InitContainerStatuses {
		switch {
		case container.State.Terminated != nil && container.State.Terminated.ExitCode == 0:
			continue
		case container.State.Terminated != nil:
			// Initialization failed
			if len(container.State.Terminated.Reason) == 0 {
				if container.State.Terminated.Signal != 0 {
					reason = fmt.Sprintf("Init container %s is terminated with signal: %d", container.Name, container.State.Terminated.Signal)
				} else {
					reason = fmt.Sprintf("Init container %s is terminated with non-zero exit code %d: %s", container.Name, container.State.Terminated.ExitCode, container.State.Terminated.Reason)
				}
			} else {
				reason = fmt.Sprintf("Init container %s is terminated. Reason %s: ", container.Name, container.State.Terminated.Reason)
			}
			state = Critical
			initializing = true
		case container.State.Waiting != nil && len(container.State.Waiting.Reason) > 0 && container.State.Waiting.Reason != "PodInitializing":
			state = Critical
			reason = fmt.Sprintf("Init container %s is waiting: %s", container.Name, container.State.Waiting.Reason)
			initializing = true
		default:
			initializing = true
			if container.State.Running != nil {
				duration := time.Since(container.State.Running.StartedAt.Time).Round(time.Second)
				if duration > prolongedInitializationThreshold {
					state = Critical
					reason = fmt.Sprintf("Init container %s has been initializing for too long (%d/%d, %s elapsed)", container.Name, i+1, len(pod.Spec.InitContainers), duration)
				} else {
					reason = fmt.Sprintf("Init container %s is currently initializing (%d/%d)", container.Name, i+1, len(pod.Spec.InitContainers))
				}
			}
		}
		break
	}

	if !initializing {
		hasRunning := false
		for _, container := range pod.Status.ContainerStatuses {
			if container.State.Waiting != nil && container.State.Waiting.Reason != "" && container.RestartCount >= 3 {
				state = Critical
				reason = fmt.Sprintf("Container %s is waiting and has restarted %d times: %s", container.Name, container.RestartCount, container.State.Waiting.Reason)
				continue
			} else if container.State.Terminated != nil {
				if container.State.Terminated.Reason == "Completed" {
					state = Ok
					reason = fmt.Sprintf("Container %s has completed successfully", container.Name)
				} else if container.State.Terminated.Reason != "" {
					state = Critical
					reason = fmt.Sprintf("Container %s is terminated. Reason: %s", container.Name, container.State.Terminated.Reason)
				} else {
					if container.State.Terminated.Signal != 0 {
						state = Critical
						reason = fmt.Sprintf("Container %s is terminated with signal: %d", container.Name, container.State.Terminated.Signal)
					} else if container.State.Terminated.ExitCode != 0 {
						state = Critical
						reason = fmt.Sprintf("Container %s is terminated with non-zero exit code %d: %s", container.Name, container.State.Terminated.ExitCode, container.State.Terminated.Reason)
					} else {
						state = Ok
						reason = fmt.Sprintf("Container %s is terminated normally", container.Name)
					}
				}
			} else if container.State.Running != nil && container.Ready {
				readyContainers++
				hasRunning = true
				state = Ok
				reason = fmt.Sprintf("Container %s is running", container.Name)
			}
		}

		if reason == "Completed" && hasRunning {
			for _, condition := range pod.Status.Conditions {
				if pod.Status.Phase == kcorev1.PodRunning {
					if condition.Type == kcorev1.PodReady && condition.Status == kcorev1.ConditionTrue {
						state = Ok
						reason = fmt.Sprintf("Pod %s/%s is %s", pod.Namespace, pod.Name, string(kcorev1.PodRunning))
					} else {
						state = Critical
						reason = fmt.Sprintf("Pod %s/%s is not ready", pod.Namespace, pod.Name)
					}
				}
			}
		}
	}

	if readyContainers == len(pod.Spec.Containers) {
		state = Ok
		reason = "All containers are ready"
	}

	return state, reason
}

func isPodInitializedConditionTrue(status *kcorev1.PodStatus) bool {
	for _, condition := range status.Conditions {
		if condition.Type != kcorev1.PodInitialized {
			continue
		}

		return condition.Status == kcorev1.ConditionTrue
	}
	return false
}

func (p *Pod) Relations() []database.Relation {
	fk := database.WithForeignKey("pod_uuid")

	return []database.Relation{
		database.HasMany(p.Conditions, fk),
		database.HasMany(p.Containers, database.WithoutCascadeDelete()),
		database.HasMany(p.Owners, fk),
		database.HasMany(p.Labels, database.WithoutCascadeDelete()),
		database.HasMany(p.PodLabels, fk),
		database.HasMany(p.Annotations, database.WithoutCascadeDelete()),
		database.HasMany(p.PodAnnotations, fk),
		database.HasMany(p.Pvcs, fk),
		database.HasMany(p.Volumes, fk),
	}
}
