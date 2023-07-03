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
)

type PersistentVolume struct {
	ResourceMeta
	AccessModes      types.Bitmask[kpersistentVolumeAccessModesSize]
	Capacity         int64
	ReclaimPolicy    string
	StorageClass     string
	VolumeMode       sql.NullString
	VolumeSourceType string
	VolumeSource     string
	Phase            string
	Reason           string
	Message          string
	Claim            *PersistentVolumeClaimRef `json:"-" db:"-"`
}

type PersistentVolumeMeta struct {
	contracts.Meta
	PersistentVolumeId types.Binary
}

func (pvm *PersistentVolumeMeta) Fingerprint() contracts.FingerPrinter {
	return pvm
}

func (pvm *PersistentVolumeMeta) ParentID() types.Binary {
	return pvm.PersistentVolumeId
}

type PersistentVolumeClaimRef struct {
	PersistentVolumeMeta
	Kind string
	Name string
	Uid  ktypes.UID
}

func NewPersistentVolume() contracts.Entity {
	return &PersistentVolume{}
}

func (p *PersistentVolume) Obtain(k8s kmetav1.Object) {
	p.ObtainMeta(k8s)

	persistentVolume := k8s.(*kcorev1.PersistentVolume)

	p.Id = types.Checksum(persistentVolume.Namespace + "/" + persistentVolume.Name)
	p.AccessModes = persistentVolumeAccessModes.Bitmask(persistentVolume.Spec.AccessModes...)
	p.Capacity = persistentVolume.Spec.Capacity.Storage().MilliValue()
	p.ReclaimPolicy = strcase.Snake(string(persistentVolume.Spec.PersistentVolumeReclaimPolicy))
	p.StorageClass = persistentVolume.Spec.StorageClassName
	p.Phase = strcase.Snake(string(persistentVolume.Status.Phase))
	p.Reason = persistentVolume.Status.Reason
	p.Message = persistentVolume.Status.Message
	if persistentVolume.Spec.VolumeMode != nil {
		p.VolumeMode = sql.NullString{
			String: string(*persistentVolume.Spec.VolumeMode),
			Valid:  true,
		}
	}
	var err error
	p.VolumeSourceType, p.VolumeSource, err = MarshalFirstNonNilStructFieldToJSON(persistentVolume.Spec.PersistentVolumeSource)
	if err != nil {
		panic(err)
	}

	p.PropertiesChecksum = types.Checksum(MustMarshalJSON(p))

	p.Claim = &PersistentVolumeClaimRef{
		PersistentVolumeMeta: PersistentVolumeMeta{
			PersistentVolumeId: p.Id,
			Meta:               contracts.Meta{Id: types.Checksum(types.MustPackSlice(p.Id, persistentVolume.Spec.ClaimRef.UID))},
		},
		Kind: persistentVolume.Spec.ClaimRef.Kind,
		Name: persistentVolume.Spec.ClaimRef.Name,
		Uid:  persistentVolume.Spec.ClaimRef.UID,
	}
	p.Claim.PropertiesChecksum = types.Checksum(MustMarshalJSON(p.Claim))
}

func (p *PersistentVolume) Relations() []database.Relation {
	fk := database.WithForeignKey("persistent_volume_id")

	return []database.Relation{
		database.HasOne(p.Claim, fk),
	}
}
