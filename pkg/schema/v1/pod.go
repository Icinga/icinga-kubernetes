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
)

type PodFactory struct {
	clientset *kubernetes.Clientset
}

type Pod struct {
	Meta
	NodeName          sql.NullString
	NominatedNodeName sql.NullString
	Ip                sql.NullString
	Phase             string
	IcingaState       IcingaState
	IcingaStateReason string
	CpuLimits         sql.NullInt64
	CpuRequests       sql.NullInt64
	MemoryLimits      sql.NullInt64
	MemoryRequests    sql.NullInt64
	Reason            sql.NullString
	Message           sql.NullString
	Qos               sql.NullString
	RestartPolicy     string
	Yaml              string
	Conditions        []PodCondition  `db:"-"`
	Containers        []*Container    `db:"-"`
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

	p.NodeName = NewNullableString(pod.Spec.NodeName)
	p.NominatedNodeName = NewNullableString(pod.Status.NominatedNodeName)
	p.Ip = NewNullableString(pod.Status.PodIP)
	p.Phase = string(pod.Status.Phase)
	p.Reason = NewNullableString(pod.Status.Reason)
	p.Message = NewNullableString(pod.Status.Message)
	p.RestartPolicy = string(pod.Spec.RestartPolicy)
	p.Qos = NewNullableString(string(pod.Status.QOSClass))

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

	p.Containers = NewContainers[Container](p, pod.Spec.Containers, pod.Status.ContainerStatuses, NewContainer)

	p.IcingaState, p.IcingaStateReason = p.getIcingaState(pod)

	for _, container := range pod.Spec.Containers {
		if !container.Resources.Limits.Cpu().IsZero() {
			p.CpuLimits.Int64 += container.Resources.Limits.Cpu().MilliValue()
			p.CpuLimits.Valid = true
		}

		if !container.Resources.Requests.Cpu().IsZero() {
			p.CpuRequests.Int64 += container.Resources.Requests.Cpu().MilliValue()
			p.CpuRequests.Valid = true
		}

		if !container.Resources.Limits.Memory().IsZero() {
			p.MemoryLimits.Int64 += container.Resources.Limits.Memory().MilliValue()
			p.MemoryLimits.Valid = true
		}

		if !container.Resources.Requests.Memory().IsZero() {
			p.MemoryRequests.Int64 += container.Resources.Requests.Memory().MilliValue()
			p.MemoryRequests.Valid = true
		}
	}

	// https://kubernetes.io/docs/concepts/workloads/pods/init-containers/#resources
	for _, container := range pod.Spec.InitContainers {
		// Init container must complete successfully before the next one starts,
		// so we don't have to sum their resources.
		if !container.Resources.Limits.Cpu().IsZero() {
			p.CpuLimits.Int64 = MaxInt(p.CpuLimits.Int64, container.Resources.Limits.Cpu().MilliValue())
			p.CpuLimits.Valid = true
		}

		if !container.Resources.Requests.Cpu().IsZero() {
			p.CpuRequests.Int64 = MaxInt(p.CpuRequests.Int64, container.Resources.Requests.Cpu().MilliValue())
			p.CpuRequests.Valid = true
		}

		if !container.Resources.Limits.Memory().IsZero() {
			p.MemoryLimits.Int64 = MaxInt(p.MemoryLimits.Int64, container.Resources.Limits.Memory().MilliValue())
			p.MemoryLimits.Valid = true
		}

		if !container.Resources.Requests.Memory().IsZero() {
			p.MemoryRequests.Int64 = MaxInt(p.MemoryRequests.Int64, container.Resources.Requests.Memory().MilliValue())
			p.MemoryRequests.Valid = true
		}
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
			OwnerUuid: EnsureUUID(ownerReference.UID),
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
	if pod.DeletionTimestamp != nil {
		if pod.Status.Reason == "NodeLost" {
			return Unknown, ""
		}
		// TODO(el): Return Critical if pod.DeletionTimestamp + pod.DeletionGracePeriodSeconds > now.
		return Ok, fmt.Sprintf("Pod %s/%s is being deleted.", pod.Namespace, pod.Name)
	}

	if PodIsEvicted(pod) {
		return Critical, fmt.Sprintf(
			"Pod %s/%s is terminal: %s: %s.", pod.Namespace, pod.Name, pod.Status.Reason, removeTrailingWhitespaceAndFullStop(pod.Status.Message))
	}

	podConditions := make(map[kcorev1.PodConditionType]kcorev1.PodCondition)
	for _, condition := range pod.Status.Conditions {
		podConditions[condition.Type] = condition
	}

	if evicted, ok := podConditions[kcorev1.DisruptionTarget]; ok {
		return Critical, fmt.Sprintf(
			"Pod %s/%s is terminal: %s: %s.", pod.Namespace, pod.Name, evicted.Reason, removeTrailingWhitespaceAndFullStop(evicted.Message))
	}

	if pod.Status.Phase == kcorev1.PodSucceeded {
		return Ok, fmt.Sprintf(
			"Pod %s/%s is succeeded as all its containers have been terminated successfully and"+
				" will not be restarted.",
			pod.Namespace, pod.Name)
	}

	if podConditions[kcorev1.PodScheduled].Status == kcorev1.ConditionFalse {
		return Critical, fmt.Sprintf(
			"Pod %s/%s can't be scheduled: %s: %s.",
			pod.Namespace,
			pod.Name,
			podConditions[kcorev1.PodScheduled].Reason,
			podConditions[kcorev1.PodScheduled].Message)
	}

	if podConditions[kcorev1.PodReadyToStartContainers].Status == kcorev1.ConditionFalse {
		return Critical, fmt.Sprintf(
			"Pod %s/%s is not ready to start containers.", pod.Namespace, pod.Name)
	}

	if pod.Status.Phase == kcorev1.PodFailed {
		// TODO(el): For each container, check its state to provide more context about the termination.
		return Critical, fmt.Sprintf(
			"Pod %s/%s has failed as all its containers have been terminated, but at least one container has failed.",
			pod.Namespace, pod.Name)
	}

	if podConditions[kcorev1.PodInitialized].Status == kcorev1.ConditionFalse {
		initContainers := NewContainers[InitContainer](
			p, pod.Spec.InitContainers, pod.Status.InitContainerStatuses, NewInitContainer)
		state := Ok
		reasons := make([]string, 0, len(initContainers))
		for _, c := range initContainers {
			state = max(state, c.IcingaState)
			reasons = append(reasons, fmt.Sprintf(
				"[%s] %s", strings.ToUpper(c.IcingaState.String()), c.IcingaStateReason))
		}

		if len(reasons) == 1 {
			// Remove square brackets state information.
			_, reason, _ := strings.Cut(reasons[0], " ")

			return state, reason
		}

		return state, fmt.Sprintf("%s/%s is %s.\n%s", pod.Namespace, pod.Name, state, strings.Join(reasons, "\n"))
	}

	var notRunning int
	state := Ok
	reasons := make([]string, 0, len(p.Containers))
	for _, c := range p.Containers {
		if c.IcingaState != Ok {
			state = max(state, c.IcingaState)
			notRunning++
		}
		reasons = append(reasons, fmt.Sprintf(
			"[%s] %s", strings.ToUpper(c.IcingaState.String()), c.IcingaStateReason))
	}

	if len(reasons) == 1 {
		// Remove square brackets state information.
		_, reason, _ := strings.Cut(reasons[0], " ")

		return state, reason
	}

	return state, fmt.Sprintf(
		"%s/%s is %s with %d out %d containers running.\n%s",
		pod.Namespace,
		pod.Name,
		state,
		len(p.Containers)-notRunning,
		len(p.Containers),
		strings.Join(reasons, "\n"))
}

func NewContainers[T any](
	p *Pod,
	containers []kcorev1.Container,
	statuses []kcorev1.ContainerStatus,
	factory func(types.UUID, kcorev1.Container, kcorev1.ContainerStatus) *T,
) []*T {
	obtained := make([]*T, 0, len(containers))

	statusesIdx := make(map[string]kcorev1.ContainerStatus, len(containers))
	for _, status := range statuses {
		statusesIdx[status.Name] = status
	}

	for _, container := range containers {
		obtained = append(obtained, factory(p.Uuid, container, statusesIdx[container.Name]))
	}

	return obtained
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

// PodIsEvicted returns true if the reported pod status is due to an eviction.
func PodIsEvicted(pod *kcorev1.Pod) bool {
	// Reason is the reason reported back in status.
	const Reason = "Evicted"

	return pod.Status.Phase == kcorev1.PodFailed && pod.Status.Reason == Reason
}

func removeTrailingWhitespaceAndFullStop(input string) string {
	trimmed := strings.TrimSpace(input)

	if strings.HasSuffix(trimmed, ".") {
		return trimmed[:len(trimmed)-1]
	}

	return trimmed
}
