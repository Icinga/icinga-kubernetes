package v1

import (
	"database/sql"
	"strings"

	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/kdatabase"
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
	StorageClass     sql.NullString
	VolumeMode       string
	VolumeSourceType string
	VolumeSource     string
	Phase            string
	Reason           sql.NullString
	Message          sql.NullString
	Yaml             string
	Claim            *PersistentVolumeClaimRef `db:"-"`
	Labels           []Label                   `db:"-"`
	Annotations      []Annotation              `db:"-"`
	Favorites        []Favorite                `db:"-"`
}

type PersistentVolumeClaimRef struct {
	kdatabase.EntityStub
	PersistentVolumeUuid types.UUID
	Kind                 string
	Name                 string
	Uid                  ktypes.UID
}

func (p PersistentVolumeClaimRef) ID() database.ID {
	return compoundId{id: p.PersistentVolumeUuid.String() + string(p.Uid)}
}

func NewPersistentVolume() Resource {
	return &PersistentVolume{}
}

func (p *PersistentVolume) Obtain(k8s kmetav1.Object, clusterUuid types.UUID) {
	p.ObtainMeta(k8s, clusterUuid)

	persistentVolume := k8s.(*kcorev1.PersistentVolume)

	p.AccessModes = persistentVolumeAccessModes.Bitmask(persistentVolume.Spec.AccessModes...)
	p.Capacity = persistentVolume.Spec.Capacity.Storage().MilliValue()
	p.ReclaimPolicy = string(persistentVolume.Spec.PersistentVolumeReclaimPolicy)
	p.StorageClass = NewNullableString(persistentVolume.Spec.StorageClassName)
	p.Phase = string(persistentVolume.Status.Phase)
	p.Reason = NewNullableString(persistentVolume.Status.Reason)
	p.Message = NewNullableString(persistentVolume.Status.Message)
	var volumeMode string
	if persistentVolume.Spec.VolumeMode != nil {
		volumeMode = string(*persistentVolume.Spec.VolumeMode)
	} else {
		volumeMode = string(kcorev1.PersistentVolumeFilesystem)
	}
	p.VolumeMode = volumeMode

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

	for labelName, labelValue := range persistentVolume.Labels {
		labelUuid := NewUUID(p.Uuid, strings.ToLower(labelName+":"+labelValue))
		p.Labels = append(p.Labels, Label{
			Uuid:         labelUuid,
			ResourceUuid: p.Uuid,
			Name:         labelName,
			Value:        labelValue,
		})
	}

	for annotationName, annotationValue := range persistentVolume.Annotations {
		annotationUuid := NewUUID(p.Uuid, strings.ToLower(annotationName+":"+annotationValue))
		p.Annotations = append(p.Annotations, Annotation{
			Uuid:         annotationUuid,
			ResourceUuid: p.Uuid,
			Name:         annotationName,
			Value:        annotationValue,
		})
	}

	scheme := kruntime.NewScheme()
	_ = kcorev1.AddToScheme(scheme)
	codec := kserializer.NewCodecFactory(scheme).EncoderForVersion(kjson.NewYAMLSerializer(kjson.DefaultMetaFactory, scheme, scheme), kcorev1.SchemeGroupVersion)
	output, _ := kruntime.Encode(codec, persistentVolume)
	p.Yaml = string(output)
}

func (p *PersistentVolume) Relations() []kdatabase.Relation {
	fk := kdatabase.WithForeignKey("persistent_volume_uuid")

	return []kdatabase.Relation{
		kdatabase.HasOne(p.Claim, fk),
		kdatabase.HasMany(p.Labels, kdatabase.WithForeignKey("resource_uuid")),
		kdatabase.HasMany(p.Annotations, kdatabase.WithForeignKey("resource_uuid")),
		kdatabase.HasMany(p.Favorites, kdatabase.WithForeignKey("resource_uuid")),
	}
}
