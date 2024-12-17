package v1

import (
	"database/sql"
	"github.com/icinga/icinga-go-library/strcase"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	kserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"strings"
)

type kpersistentVolumeAccessModesSize byte

type kpersistentVolumeAccessModes map[kcorev1.PersistentVolumeAccessMode]kpersistentVolumeAccessModesSize

func (modes kpersistentVolumeAccessModes) Bitmask(mode ...kcorev1.PersistentVolumeAccessMode) Bitmask[kpersistentVolumeAccessModesSize] {
	b := Bitmask[kpersistentVolumeAccessModesSize]{}

	for _, m := range mode {
		b.Set(modes[m])
	}

	return b
}

var persistentVolumeAccessModes = kpersistentVolumeAccessModes{
	kcorev1.ReadWriteOnce:    1 << 0,
	kcorev1.ReadOnlyMany:     1 << 1,
	kcorev1.ReadWriteMany:    1 << 2,
	kcorev1.ReadWriteOncePod: 1 << 3,
}

type Pvc struct {
	Meta
	DesiredAccessModes  Bitmask[kpersistentVolumeAccessModesSize]
	ActualAccessModes   Bitmask[kpersistentVolumeAccessModesSize]
	MinimumCapacity     sql.NullInt64
	ActualCapacity      sql.NullInt64
	Phase               string
	VolumeName          sql.NullString
	VolumeMode          string
	StorageClass        sql.NullString
	Yaml                string
	Conditions          []PvcCondition       `db:"-"`
	Labels              []Label              `db:"-"`
	PvcLabels           []PvcLabel           `db:"-"`
	ResourceLabels      []ResourceLabel      `db:"-"`
	Annotations         []Annotation         `db:"-"`
	PvcAnnotations      []PvcAnnotation      `db:"-"`
	ResourceAnnotations []ResourceAnnotation `db:"-"`
}

type PvcCondition struct {
	PvcUuid        types.UUID
	Type           string
	Status         string
	LastProbe      types.UnixMilli
	LastTransition types.UnixMilli
	Reason         string
	Message        string
}

type PvcLabel struct {
	PvcUuid   types.UUID
	LabelUuid types.UUID
}

type PvcAnnotation struct {
	PvcUuid        types.UUID
	AnnotationUuid types.UUID
}

func NewPvc() Resource {
	return &Pvc{}
}

func (p *Pvc) Obtain(k8s kmetav1.Object, clusterUuid types.UUID) {
	p.ObtainMeta(k8s, clusterUuid)

	pvc := k8s.(*kcorev1.PersistentVolumeClaim)

	p.DesiredAccessModes = persistentVolumeAccessModes.Bitmask(pvc.Spec.AccessModes...)
	p.ActualAccessModes = persistentVolumeAccessModes.Bitmask(pvc.Status.AccessModes...)
	if storageRequest, ok := pvc.Spec.Resources.Requests[kcorev1.ResourceStorage]; ok {
		p.MinimumCapacity = sql.NullInt64{
			Int64: storageRequest.MilliValue(),
			Valid: true,
		}
	}
	if actualStorage, ok := pvc.Status.Capacity[kcorev1.ResourceStorage]; ok {
		p.ActualCapacity = sql.NullInt64{
			Int64: actualStorage.MilliValue(),
			Valid: true,
		}
	}
	p.Phase = string(pvc.Status.Phase)
	p.VolumeName = NewNullableString(pvc.Spec.VolumeName)
	var volumeMode string
	if pvc.Spec.VolumeMode != nil {
		volumeMode = string(*pvc.Spec.VolumeMode)
	} else {
		volumeMode = string(kcorev1.PersistentVolumeFilesystem)
	}
	p.VolumeMode = volumeMode
	p.StorageClass = NewNullableString(pvc.Spec.StorageClassName)

	for _, condition := range pvc.Status.Conditions {
		p.Conditions = append(p.Conditions, PvcCondition{
			PvcUuid:        p.Uuid,
			Type:           strcase.Snake(string(condition.Type)),
			Status:         string(condition.Status),
			LastProbe:      types.UnixMilli(condition.LastProbeTime.Time),
			LastTransition: types.UnixMilli(condition.LastTransitionTime.Time),
			Reason:         condition.Reason,
			Message:        condition.Message,
		})
	}

	for labelName, labelValue := range pvc.Labels {
		labelUuid := NewUUID(p.Uuid, strings.ToLower(labelName+":"+labelValue))
		p.Labels = append(p.Labels, Label{
			Uuid:  labelUuid,
			Name:  labelName,
			Value: labelValue,
		})
		p.PvcLabels = append(p.PvcLabels, PvcLabel{
			PvcUuid:   p.Uuid,
			LabelUuid: labelUuid,
		})
		p.ResourceLabels = append(p.ResourceLabels, ResourceLabel{
			ResourceUuid: p.Uuid,
			LabelUuid:    labelUuid,
		})
	}

	for annotationName, annotationValue := range pvc.Annotations {
		annotationUuid := NewUUID(p.Uuid, strings.ToLower(annotationName+":"+annotationValue))
		p.Annotations = append(p.Annotations, Annotation{
			Uuid:  annotationUuid,
			Name:  annotationName,
			Value: annotationValue,
		})
		p.PvcAnnotations = append(p.PvcAnnotations, PvcAnnotation{
			PvcUuid:        p.Uuid,
			AnnotationUuid: annotationUuid,
		})
		p.PvcAnnotations = append(p.PvcAnnotations, PvcAnnotation{
			PvcUuid:        p.Uuid,
			AnnotationUuid: annotationUuid,
		})
	}

	scheme := kruntime.NewScheme()
	_ = kcorev1.AddToScheme(scheme)
	codec := kserializer.NewCodecFactory(scheme).EncoderForVersion(kjson.NewYAMLSerializer(kjson.DefaultMetaFactory, scheme, scheme), kcorev1.SchemeGroupVersion)
	output, _ := kruntime.Encode(codec, pvc)
	p.Yaml = string(output)
}

func (p *Pvc) Relations() []database.Relation {
	fk := database.WithForeignKey("pvc_uuid")

	return []database.Relation{
		database.HasMany(p.Conditions, fk),
		database.HasMany(p.ResourceLabels, database.WithForeignKey("resource_uuid")),
		database.HasMany(p.Labels, database.WithoutCascadeDelete()),
		database.HasMany(p.PvcLabels, fk),
		database.HasMany(p.ResourceAnnotations, database.WithForeignKey("resource_uuid")),
		database.HasMany(p.Annotations, database.WithoutCascadeDelete()),
		database.HasMany(p.PvcAnnotations, fk),
	}
}
