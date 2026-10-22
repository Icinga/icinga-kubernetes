package v1

import (
	"database/sql"
	"strings"

	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/strcase"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/kdatabase"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	Favorites           []Favorite           `db:"-"`
}

type PvcCondition struct {
	kdatabase.EntityStub
	PvcUuid        types.UUID
	Type           string
	Status         string
	LastProbe      types.UnixMilli
	LastTransition types.UnixMilli
	Reason         string
	Message        string
}

func (p PvcCondition) ID() database.ID {
	return compoundId{id: p.PvcUuid.String() + p.Type}
}

type PvcLabel struct {
	kdatabase.EntityStub
	PvcUuid   types.UUID
	LabelUuid types.UUID
}

func (p PvcLabel) ID() database.ID {
	return compoundId{id: p.PvcUuid.String() + p.LabelUuid.String()}
}

type PvcAnnotation struct {
	kdatabase.EntityStub
	PvcUuid        types.UUID
	AnnotationUuid types.UUID
}

func (p PvcAnnotation) ID() database.ID {
	return compoundId{id: p.PvcUuid.String() + p.AnnotationUuid.String()}
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
		p.ResourceAnnotations = append(p.ResourceAnnotations, ResourceAnnotation{
			ResourceUuid:   p.Uuid,
			AnnotationUuid: annotationUuid,
		})
	}

	p.Yaml = MarshalYaml(pvc, kcorev1.SchemeGroupVersion, kcorev1.AddToScheme)
}

func (p *Pvc) Relations() []kdatabase.Relation {
	fk := kdatabase.WithForeignKey("pvc_uuid")

	return []kdatabase.Relation{
		kdatabase.HasMany(p.Conditions, fk),
		kdatabase.HasMany(p.ResourceLabels, kdatabase.WithForeignKey("resource_uuid")),
		kdatabase.HasMany(p.Labels, kdatabase.WithoutCascadeDelete()),
		kdatabase.HasMany(p.PvcLabels, fk),
		kdatabase.HasMany(p.ResourceAnnotations, kdatabase.WithForeignKey("resource_uuid")),
		kdatabase.HasMany(p.Annotations, kdatabase.WithoutCascadeDelete()),
		kdatabase.HasMany(p.PvcAnnotations, fk),
		kdatabase.HasMany(p.Favorites, kdatabase.WithForeignKey("resource_uuid")),
	}
}
