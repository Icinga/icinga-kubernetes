package v1

import (
	"github.com/icinga/icinga-kubernetes/pkg/contracts"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/strcase"
	"github.com/icinga/icinga-kubernetes/pkg/types"
	kappsv1 "k8s.io/api/apps/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DaemonSet struct {
	ResourceMeta
	UpdateStrategy         string
	MinReadySeconds        int32
	DesiredNumberScheduled int32
	CurrentNumberScheduled int32
	NumberMisscheduled     int32
	NumberReady            int32
	UpdateNumberScheduled  int32
	NumberAvailable        int32
	NumberUnavailable      int32
	Conditions             []*DaemonSetCondition `db:"-" hash:"-"`
	Labels                 []*Label              `db:"-" hash:"-"`
}

type DaemonSetMeta struct {
	contracts.Meta
	DaemonSetId types.Binary
}

func (dm *DaemonSetMeta) Fingerprint() contracts.FingerPrinter {
	return dm
}

func (dm *DaemonSetMeta) ParentID() types.Binary {
	return dm.DaemonSetId
}

type DaemonSetCondition struct {
	DaemonSetMeta
	Type           string
	Status         string
	LastTransition types.UnixMilli
	Reason         string
	Message        string
}

func NewDaemonSet() contracts.Entity {
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

	d.PropertiesChecksum = types.HashStruct(d)

	for _, condition := range daemonSet.Status.Conditions {
		daemonCond := &DaemonSetCondition{
			DaemonSetMeta: DaemonSetMeta{
				DaemonSetId: d.Id,
				Meta:        contracts.Meta{Id: types.Checksum(types.MustPackSlice(d.Id, condition.Type))},
			},
			Type:           string(condition.Type),
			Status:         string(condition.Status),
			LastTransition: types.UnixMilli(condition.LastTransitionTime.Time),
			Reason:         condition.Reason,
			Message:        condition.Message,
		}
		daemonCond.PropertiesChecksum = types.HashStruct(daemonCond)

		d.Conditions = append(d.Conditions, daemonCond)
	}

	for labelName, labelValue := range daemonSet.Labels {
		label := NewLabel(labelName, labelValue)
		label.DaemonSetId = d.Id
		label.PropertiesChecksum = types.HashStruct(label)

		d.Labels = append(d.Labels, label)
	}
}

func (d *DaemonSet) Relations() []database.Relation {
	fk := database.WithForeignKey("daemon_set_id")

	return []database.Relation{
		database.HasMany(d.Conditions, fk),
		database.HasMany(d.Labels, fk),
	}
}
