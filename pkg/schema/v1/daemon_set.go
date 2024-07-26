package v1

import (
	"fmt"
	"github.com/icinga/icinga-go-library/strcase"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	kappsv1 "k8s.io/api/apps/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	kserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	ktypes "k8s.io/apimachinery/pkg/types"
	"net/url"
	"strings"
)

type DaemonSet struct {
	Meta
	UpdateStrategy         string
	MinReadySeconds        int32
	DesiredNumberScheduled int32
	CurrentNumberScheduled int32
	NumberMisscheduled     int32
	NumberReady            int32
	UpdateNumberScheduled  int32
	NumberAvailable        int32
	NumberUnavailable      int32
	Yaml                   string
	IcingaState            IcingaState
	IcingaStateReason      string
	Conditions             []DaemonSetCondition  `db:"-"`
	Owners                 []DaemonSetOwner      `db:"-"`
	Labels                 []Label               `db:"-"`
	DaemonSetLabels        []DaemonSetLabel      `db:"-"`
	Annotations            []Annotation          `db:"-"`
	DaemonSetAnnotations   []DaemonSetAnnotation `db:"-"`
}

type DaemonSetCondition struct {
	DaemonSetUuid  types.UUID
	Type           string
	Status         string
	LastTransition types.UnixMilli
	Reason         string
	Message        string
}

type DaemonSetOwner struct {
	DaemonSetUuid      types.UUID
	OwnerUuid          types.UUID
	Kind               string
	Name               string
	Uid                ktypes.UID
	Controller         types.Bool
	BlockOwnerDeletion types.Bool
}

type DaemonSetLabel struct {
	DaemonSetUuid types.UUID
	LabelUuid     types.UUID
}

type DaemonSetAnnotation struct {
	DaemonSetUuid  types.UUID
	AnnotationUuid types.UUID
}

func NewDaemonSet() Resource {
	return &DaemonSet{}
}

func (d *DaemonSet) Obtain(k8s kmetav1.Object) {
	d.ObtainMeta(k8s)

	daemonSet := k8s.(*kappsv1.DaemonSet)

	d.UpdateStrategy = string(daemonSet.Spec.UpdateStrategy.Type)
	d.MinReadySeconds = daemonSet.Spec.MinReadySeconds
	d.DesiredNumberScheduled = daemonSet.Status.DesiredNumberScheduled
	d.CurrentNumberScheduled = daemonSet.Status.CurrentNumberScheduled
	d.NumberMisscheduled = daemonSet.Status.NumberMisscheduled
	d.NumberReady = daemonSet.Status.NumberReady
	d.UpdateNumberScheduled = daemonSet.Status.UpdatedNumberScheduled
	d.NumberAvailable = daemonSet.Status.NumberAvailable
	d.NumberUnavailable = daemonSet.Status.NumberUnavailable
	d.IcingaState, d.IcingaStateReason = d.getIcingaState()

	for _, condition := range daemonSet.Status.Conditions {
		d.Conditions = append(d.Conditions, DaemonSetCondition{
			DaemonSetUuid:  d.Uuid,
			Type:           string(condition.Type),
			Status:         string(condition.Status),
			LastTransition: types.UnixMilli(condition.LastTransitionTime.Time),
			Reason:         condition.Reason,
			Message:        condition.Message,
		})
	}

	for _, ownerReference := range daemonSet.OwnerReferences {
		var blockOwnerDeletion, controller bool
		if ownerReference.BlockOwnerDeletion != nil {
			blockOwnerDeletion = *ownerReference.BlockOwnerDeletion
		}
		if ownerReference.Controller != nil {
			controller = *ownerReference.Controller
		}
		d.Owners = append(d.Owners, DaemonSetOwner{
			DaemonSetUuid: d.Uuid,
			OwnerUuid:     EnsureUUID(ownerReference.UID),
			Kind:          strcase.Snake(ownerReference.Kind),
			Name:          ownerReference.Name,
			Uid:           ownerReference.UID,
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

	for labelName, labelValue := range daemonSet.Labels {
		labelUuid := NewUUID(d.Uuid, strings.ToLower(labelName+":"+labelValue))
		d.Labels = append(d.Labels, Label{
			Uuid:  labelUuid,
			Name:  labelName,
			Value: labelValue,
		})
		d.DaemonSetLabels = append(d.DaemonSetLabels, DaemonSetLabel{
			DaemonSetUuid: d.Uuid,
			LabelUuid:     labelUuid,
		})
	}

	for annotationName, annotationValue := range daemonSet.Annotations {
		annotationUuid := NewUUID(d.Uuid, strings.ToLower(annotationName+":"+annotationValue))
		d.Annotations = append(d.Annotations, Annotation{
			Uuid:  annotationUuid,
			Name:  annotationName,
			Value: annotationValue,
		})
		d.DaemonSetAnnotations = append(d.DaemonSetAnnotations, DaemonSetAnnotation{
			DaemonSetUuid:  d.Uuid,
			AnnotationUuid: annotationUuid,
		})
	}

	scheme := kruntime.NewScheme()
	_ = kappsv1.AddToScheme(scheme)
	codec := kserializer.NewCodecFactory(scheme).EncoderForVersion(kjson.NewYAMLSerializer(kjson.DefaultMetaFactory, scheme, scheme), kappsv1.SchemeGroupVersion)
	output, _ := kruntime.Encode(codec, daemonSet)
	d.Yaml = string(output)
}

// GetNotificationsEvent implements the notifications.Notifiable interface.
func (d *DaemonSet) GetNotificationsEvent(baseUrl *url.URL) map[string]any {
	daemonSetUrl := baseUrl.JoinPath("/daemonset")
	daemonSetUrl.RawQuery = fmt.Sprintf("id=%s", d.Uuid)

	return map[string]any{
		"name":     d.Namespace + "/" + d.Name,
		"severity": d.IcingaState.ToSeverity(),
		"message":  d.IcingaStateReason,
		"url":      daemonSetUrl.String(),
		"tags": map[string]any{
			"name":      d.Name,
			"namespace": d.Namespace,
			"uuid":      d.Uuid.String(),
			"resource":  "daemon_set",
		},
	}
}

func (d *DaemonSet) getIcingaState() (IcingaState, string) {
	if d.DesiredNumberScheduled < 1 {
		reason := fmt.Sprintf("DaemonSet %s/%s has an invalid desired node count: %d.", d.Namespace, d.Name, d.DesiredNumberScheduled)

		return Unknown, reason
	}

	switch {
	case d.NumberAvailable == 0:
		reason := fmt.Sprintf("DaemonSet %s/%s does not have a single pod available which should run on %d desired nodes.", d.Namespace, d.Name, d.DesiredNumberScheduled)

		return Critical, reason
	case d.NumberAvailable < d.DesiredNumberScheduled:
		reason := fmt.Sprintf("DaemonSet %s/%s pods are only available on %d out of %d desired nodes.", d.Namespace, d.Name, d.NumberAvailable, d.DesiredNumberScheduled)

		return Warning, reason
	default:
		reason := fmt.Sprintf("DaemonSet %s/%s has pods available on all %d desired nodes.", d.Namespace, d.Name, d.DesiredNumberScheduled)

		return Ok, reason
	}
}

func (d *DaemonSet) Relations() []database.Relation {
	fk := database.WithForeignKey("daemon_set_uuid")

	return []database.Relation{
		database.HasMany(d.Conditions, fk),
		database.HasMany(d.Owners, fk),
		database.HasMany(d.DaemonSetLabels, fk),
		database.HasMany(d.Labels, database.WithoutCascadeDelete()),
		database.HasMany(d.DaemonSetAnnotations, fk),
		database.HasMany(d.Annotations, database.WithoutCascadeDelete()),
	}
}
