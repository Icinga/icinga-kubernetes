package v1

import (
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/strcase"
	kappsv1 "k8s.io/api/apps/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	kserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
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
	Conditions             []DaemonSetCondition `db:"-"`
	Labels                 []Label              `db:"-"`
	DaemonSetLabels        []DaemonSetLabel     `db:"-"`
}

type DaemonSetCondition struct {
	DaemonSetUuid  types.UUID
	Type           string
	Status         string
	LastTransition types.UnixMilli
	Reason         string
	Message        string
}

type DaemonSetLabel struct {
	DaemonSetUuid types.UUID
	LabelUuid     types.UUID
}

func NewDaemonSet() Resource {
	return &DaemonSet{}
}

func (d *DaemonSet) Obtain(k8s kmetav1.Object) {
	d.ObtainMeta(k8s)

	daemonSet := k8s.(*kappsv1.DaemonSet)

	d.UpdateStrategy = strcase.Snake(string(daemonSet.Spec.UpdateStrategy.Type))
	d.MinReadySeconds = daemonSet.Spec.MinReadySeconds
	d.DesiredNumberScheduled = daemonSet.Status.DesiredNumberScheduled
	d.CurrentNumberScheduled = daemonSet.Status.CurrentNumberScheduled
	d.NumberMisscheduled = daemonSet.Status.NumberMisscheduled
	d.NumberReady = daemonSet.Status.NumberReady
	d.UpdateNumberScheduled = daemonSet.Status.UpdatedNumberScheduled
	d.NumberAvailable = daemonSet.Status.NumberAvailable
	d.NumberUnavailable = daemonSet.Status.NumberUnavailable

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
	scheme := kruntime.NewScheme()
	_ = kappsv1.AddToScheme(scheme)
	codec := kserializer.NewCodecFactory(scheme).EncoderForVersion(kjson.NewYAMLSerializer(kjson.DefaultMetaFactory, scheme, scheme), kappsv1.SchemeGroupVersion)
	output, _ := kruntime.Encode(codec, daemonSet)
	d.Yaml = string(output)
}

func (d *DaemonSet) Relations() []database.Relation {
	fk := database.WithForeignKey("daemon_set_uuid")

	return []database.Relation{
		database.HasMany(d.Conditions, fk),
		database.HasMany(d.DaemonSetLabels, fk),
		database.HasMany(d.Labels, database.WithoutCascadeDelete()),
	}
}
