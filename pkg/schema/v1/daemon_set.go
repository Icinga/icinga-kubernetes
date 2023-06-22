package v1

import (
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/strcase"
	"github.com/icinga/icinga-kubernetes/pkg/types"
	kappsv1 "k8s.io/api/apps/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

type DaemonSet struct {
	Meta
	Id                     types.Binary
	UpdateStrategy         string
	MinReadySeconds        int32
	DesiredNumberScheduled int32
	CurrentNumberScheduled int32
	NumberMisscheduled     int32
	NumberReady            int32
	UpdateNumberScheduled  int32
	NumberAvailable        int32
	NumberUnavailable      int32
	Conditions             []DaemonSetCondition `db:"-"`
	Labels                 []Label              `db:"-"`
	DaemonSetLabels        []DaemonSetLabel     `db:"-"`
}

type DaemonSetCondition struct {
	DaemonSetId    types.Binary
	Type           string
	Status         string
	LastTransition types.UnixMilli
	Reason         string
	Message        string
}

type DaemonSetLabel struct {
	DaemonSetId types.Binary
	LabelId     types.Binary
}

func NewDaemonSet() Resource {
	return &DaemonSet{}
}

func (d *DaemonSet) Obtain(k8s kmetav1.Object) {
	d.ObtainMeta(k8s)

	daemonSet := k8s.(*kappsv1.DaemonSet)

	d.Id = types.Checksum(daemonSet.Namespace + "/" + daemonSet.Name)
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
			DaemonSetId:    d.Id,
			Type:           string(condition.Type),
			Status:         string(condition.Status),
			LastTransition: types.UnixMilli(condition.LastTransitionTime.Time),
			Reason:         condition.Reason,
			Message:        condition.Message,
		})
	}

	for labelName, labelValue := range daemonSet.Labels {
		labelId := types.Checksum(strings.ToLower(labelName + ":" + labelValue))
		d.Labels = append(d.Labels, Label{
			Id:    labelId,
			Name:  labelName,
			Value: labelValue,
		})
		d.DaemonSetLabels = append(d.DaemonSetLabels, DaemonSetLabel{
			DaemonSetId: d.Id,
			LabelId:     labelId,
		})
	}
}

func (d *DaemonSet) Relations() []database.Relation {
	fk := database.WithForeignKey("daemon_set_id")

	return []database.Relation{
		database.HasMany(d.Conditions, fk),
		database.HasMany(d.DaemonSetLabels, fk),
		database.HasMany(d.Labels, database.WithoutCascadeDelete()),
	}
}
