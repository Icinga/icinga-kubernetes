package v1

import (
	"database/sql"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	kserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	ktypes "k8s.io/apimachinery/pkg/types"
	"strings"
)

type PersistentVolume struct {
	Meta
	AccessModes                 Bitmask[kpersistentVolumeAccessModesSize]
	Capacity                    int64
	ReclaimPolicy               string
	StorageClass                sql.NullString
	VolumeMode                  string
	VolumeSourceType            string
	VolumeSource                string
	Phase                       string
	Reason                      sql.NullString
	Message                     sql.NullString
	Yaml                        string
	Claim                       *PersistentVolumeClaimRef    `db:"-"`
	Labels                      []Label                      `db:"-"`
	PersistentVolumeLabels      []PersistentVolumeLabel      `db:"-"`
	Annotations                 []Annotation                 `db:"-"`
	PersistentVolumeAnnotations []PersistentVolumeAnnotation `db:"-"`
}

type PersistentVolumeClaimRef struct {
	PersistentVolumeUuid types.UUID
	Kind                 string
	Name                 string
	Uid                  ktypes.UID
}

type PersistentVolumeLabel struct {
	PersistentVolumeUuid types.UUID
	LabelUuid            types.UUID
}

type PersistentVolumeAnnotation struct {
	PersistentVolumeUuid types.UUID
	AnnotationUuid       types.UUID
}

func NewPersistentVolume() Resource {
	return &PersistentVolume{}
}

func (p *PersistentVolume) Obtain(k8s kmetav1.Object) {
	p.ObtainMeta(k8s)

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
			Uuid:  labelUuid,
			Name:  labelName,
			Value: labelValue,
		})
		p.PersistentVolumeLabels = append(p.PersistentVolumeLabels, PersistentVolumeLabel{
			PersistentVolumeUuid: p.Uuid,
			LabelUuid:            labelUuid,
		})
	}

	for annotationName, annotationValue := range persistentVolume.Annotations {
		annotationUuid := NewUUID(p.Uuid, strings.ToLower(annotationName+":"+annotationValue))
		p.Annotations = append(p.Annotations, Annotation{
			Uuid:  annotationUuid,
			Name:  annotationName,
			Value: annotationValue,
		})
		p.PersistentVolumeAnnotations = append(p.PersistentVolumeAnnotations, PersistentVolumeAnnotation{
			PersistentVolumeUuid: p.Uuid,
			AnnotationUuid:       annotationUuid,
		})
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
		database.HasMany(p.Labels, database.WithoutCascadeDelete()),
		database.HasMany(p.PersistentVolumeLabels, fk),
		database.HasMany(p.PersistentVolumeAnnotations, fk),
		database.HasMany(p.Annotations, database.WithoutCascadeDelete()),
	}
}
