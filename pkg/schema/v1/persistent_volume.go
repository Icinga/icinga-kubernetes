package v1

import (
	"database/sql"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/strcase"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	kserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	ktypes "k8s.io/apimachinery/pkg/types"
)

type PersistentVolume struct {
	Meta
	AccessModes      Bitmask[kpersistentVolumeAccessModesSize]
	Capacity         int64
	ReclaimPolicy    string
	StorageClass     string
	VolumeMode       sql.NullString
	VolumeSourceType string
	VolumeSource     string
	Phase            string
	Reason           string
	Message          string
	Yaml             string
	Claim            *PersistentVolumeClaimRef `db:"-"`
}

type PersistentVolumeClaimRef struct {
	PersistentVolumeUuid types.UUID
	Kind                 string
	Name                 string
	Uid                  ktypes.UID
}

func NewPersistentVolume() Resource {
	return &PersistentVolume{}
}

func (p *PersistentVolume) Obtain(k8s kmetav1.Object) {
	p.ObtainMeta(k8s)

	persistentVolume := k8s.(*kcorev1.PersistentVolume)

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

	if persistentVolume.Spec.ClaimRef != nil {
		p.Claim = &PersistentVolumeClaimRef{
			PersistentVolumeUuid: p.Uuid,
			Kind:                 persistentVolume.Spec.ClaimRef.Kind,
			Name:                 persistentVolume.Spec.ClaimRef.Name,
			Uid:                  persistentVolume.Spec.ClaimRef.UID,
		}
	}

	scheme := kruntime.NewScheme()
	_ = kcorev1.AddToScheme(scheme)
	codec := kserializer.NewCodecFactory(scheme).EncoderForVersion(kjson.NewYAMLSerializer(kjson.DefaultMetaFactory, scheme, scheme), kcorev1.SchemeGroupVersion)
	output, _ := kruntime.Encode(codec, persistentVolume)
	p.Yaml = string(output)
}

func (p *PersistentVolume) Relations() []database.Relation {
	if p.Claim == nil {
		return []database.Relation{}
	}

	fk := database.WithForeignKey("persistent_volume_uuid")

	return []database.Relation{
		database.HasOne(p.Claim, fk),
	}
}
