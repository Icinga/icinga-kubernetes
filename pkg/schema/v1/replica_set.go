package v1

import (
	"github.com/icinga/icinga-kubernetes/pkg/contracts"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/strcase"
	"github.com/icinga/icinga-kubernetes/pkg/types"
	kappsv1 "k8s.io/api/apps/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
)

type ReplicaSet struct {
	ResourceMeta
	DesiredReplicas      int32
	MinReadySeconds      int32
	ActualReplicas       int32
	FullyLabeledReplicas int32
	ReadyReplicas        int32
	AvailableReplicas    int32
	Conditions           []*ReplicaSetCondition `json:"-" db:"-"`
	Owners               []*ReplicaSetOwner     `json:"-" db:"-"`
	Labels               []*Label               `json:"-" db:"-"`
}

type ReplicaSetMeta struct {
	contracts.Meta
	ReplicaSetId types.Binary
}

func (rm *ReplicaSetMeta) Fingerprint() contracts.FingerPrinter {
	return rm
}

func (rm *ReplicaSetMeta) ParentID() types.Binary {
	return rm.ReplicaSetId
}

type ReplicaSetCondition struct {
	ReplicaSetMeta
	Type           string
	Status         string
	LastTransition types.UnixMilli
	Reason         string
	Message        string
}

type ReplicaSetOwner struct {
	ReplicaSetMeta
	Kind               string
	Name               string
	Uid                ktypes.UID
	Controller         types.Bool
	BlockOwnerDeletion types.Bool
}

func NewReplicaSet() contracts.Entity {
	return &ReplicaSet{}
}

func (r *ReplicaSet) Obtain(k8s kmetav1.Object) {
	r.ObtainMeta(k8s)

	replicaSet := k8s.(*kappsv1.ReplicaSet)

	var desiredReplicas int32
	if replicaSet.Spec.Replicas != nil {
		desiredReplicas = *replicaSet.Spec.Replicas
	}
	r.Id = types.Checksum(r.Namespace + "/" + r.Name)
	r.DesiredReplicas = desiredReplicas
	r.MinReadySeconds = replicaSet.Spec.MinReadySeconds
	r.ActualReplicas = replicaSet.Status.Replicas
	r.FullyLabeledReplicas = replicaSet.Status.FullyLabeledReplicas
	r.ReadyReplicas = replicaSet.Status.ReadyReplicas
	r.AvailableReplicas = replicaSet.Status.AvailableReplicas

	r.PropertiesChecksum = types.Checksum(MustMarshalJSON(r))

	for _, condition := range replicaSet.Status.Conditions {
		replicaSetCond := &ReplicaSetCondition{
			ReplicaSetMeta: ReplicaSetMeta{
				ReplicaSetId: r.Id,
				Meta:         contracts.Meta{Id: types.Checksum(types.MustPackSlice(r.Id, condition.Type))},
			},
			Type:           strcase.Snake(string(condition.Type)),
			Status:         string(condition.Status),
			LastTransition: types.UnixMilli(condition.LastTransitionTime.Time),
			Reason:         condition.Reason,
			Message:        condition.Message,
		}
		replicaSetCond.PropertiesChecksum = types.Checksum(MustMarshalJSON(replicaSetCond))

		r.Conditions = append(r.Conditions, replicaSetCond)
	}

	for _, ownerReference := range replicaSet.OwnerReferences {
		var blockOwnerDeletion, controller bool
		if ownerReference.BlockOwnerDeletion != nil {
			blockOwnerDeletion = *ownerReference.BlockOwnerDeletion
		}
		if ownerReference.Controller != nil {
			controller = *ownerReference.Controller
		}

		owner := &ReplicaSetOwner{
			ReplicaSetMeta: ReplicaSetMeta{
				ReplicaSetId: r.Id,
				Meta:         contracts.Meta{Id: types.Checksum(types.MustPackSlice(r.Id, ownerReference.UID))},
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

		r.Owners = append(r.Owners, owner)
	}

	for labelName, labelValue := range replicaSet.Labels {
		label := NewLabel(labelName, labelValue)
		label.ReplicaSetId = r.Id
		label.PropertiesChecksum = types.Checksum(MustMarshalJSON(label))

		r.Labels = append(r.Labels, label)
	}
}

func (r *ReplicaSet) Relations() []database.Relation {
	fk := database.WithForeignKey("replica_set_id")

	return []database.Relation{
		database.HasMany(r.Conditions, fk),
		database.HasMany(r.Owners, fk),
		database.HasMany(r.Labels, fk),
	}
}
