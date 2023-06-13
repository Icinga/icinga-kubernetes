package v1

import (
	"database/sql"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/strcase"
	"github.com/icinga/icinga-kubernetes/pkg/types"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
)

type PersistentVolume struct {
	Meta
	Id               types.Binary
	Capacity         int64
	Phase            string
	Reason           string
	Message          string
	AccessModes      types.Bitmask[kpersistentVolumeAccessModesSize]
	VolumeMode       sql.NullString
	StorageClass     string
	VolumeSourceType string
	VolumeSource     string
	ReclaimPolicy    string
	Claims           []PersistentVolumeClaimRef `db:"-"`
}

type PersistentVolumeClaimRef struct {
	PersistentVolumeId types.Binary
	Kind               string
	Name               string
	Uid                ktypes.UID
}

func NewPersistentVolume() Resource {
	return &PersistentVolume{}
}

func (p *PersistentVolume) Obtain(k8s kmetav1.Object) {
	p.ObtainMeta(k8s)

	persistentVolume := k8s.(*kcorev1.PersistentVolume)

	p.Id = types.Checksum(persistentVolume.Namespace + "/" + persistentVolume.Name)
	p.Capacity = persistentVolume.Spec.Capacity.Storage().MilliValue()
	p.Phase = strcase.Snake(string(persistentVolume.Status.Phase))
	p.Reason = persistentVolume.Status.Reason
	p.Message = persistentVolume.Status.Message
	p.AccessModes = persistentVolumeAccessModes.Bitmask(persistentVolume.Spec.AccessModes...)
	p.Claims = append(p.Claims, PersistentVolumeClaimRef{
		PersistentVolumeId: p.Id,
		Kind:               persistentVolume.Spec.ClaimRef.Kind,
		Name:               persistentVolume.Spec.ClaimRef.Name,
		Uid:                persistentVolume.Spec.ClaimRef.UID,
	})
	if persistentVolume.Spec.VolumeMode != nil {
		p.VolumeMode = sql.NullString{
			String: string(*persistentVolume.Spec.VolumeMode),
			Valid:  true,
		}
	}
	p.StorageClass = persistentVolume.Spec.StorageClassName

	var err error
	p.VolumeSourceType, p.VolumeSource, err = MarshalFirstNonNilStructFieldToJSON(persistentVolume.Spec.PersistentVolumeSource)
	if err != nil {
		panic(err)
	}
}

func (p *PersistentVolume) Relations() database.Relations {
	return database.Relations{
		database.HasMany[PersistentVolumeClaimRef]{
			Entities:    p.Claims,
			ForeignKey_: "persistent_volume_id",
		},
	}
}
