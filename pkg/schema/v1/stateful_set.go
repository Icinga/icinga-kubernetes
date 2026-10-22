package v1

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/strcase"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/kdatabase"
	"github.com/icinga/icinga-kubernetes/pkg/notifications"
	kappsv1 "k8s.io/api/apps/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	kserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	ktypes "k8s.io/apimachinery/pkg/types"
)

type StatefulSet struct {
	Meta
	DesiredReplicas                                 int32
	ServiceName                                     string
	PodManagementPolicy                             string
	UpdateStrategy                                  string
	MinReadySeconds                                 int32
	PersistentVolumeClaimRetentionPolicyWhenDeleted string
	PersistentVolumeClaimRetentionPolicyWhenScaled  string
	Ordinals                                        int32
	ActualReplicas                                  int32
	ReadyReplicas                                   int32
	CurrentReplicas                                 int32
	UpdatedReplicas                                 int32
	AvailableReplicas                               int32
	Yaml                                            string
	IcingaState                                     IcingaState
	IcingaStateReason                               string
	Conditions                                      []StatefulSetCondition `db:"-"`
	Owners                                          []StatefulSetOwner     `db:"-"`
	Labels                                          []Label                `db:"-"`
	ResourceLabels                                  []ResourceLabel        `db:"-"`
	Annotations                                     []Annotation           `db:"-"`
	ResourceAnnotations                             []ResourceAnnotation   `db:"-"`
	Favorites                                       []Favorite             `db:"-"`
}

type StatefulSetCondition struct {
	kdatabase.EntityStub
	StatefulSetUuid types.UUID
	Type            string
	Status          string
	LastTransition  types.UnixMilli
	Reason          string
	Message         string
}

func (s StatefulSetCondition) ID() database.ID {
	return compoundId{id: s.StatefulSetUuid.String() + s.Type}
}

type StatefulSetOwner struct {
	kdatabase.EntityStub
	StatefulSetUuid    types.UUID
	OwnerUuid          types.UUID
	Kind               string
	Name               string
	Uid                ktypes.UID
	Controller         types.Bool
	BlockOwnerDeletion types.Bool
}

func (s StatefulSetOwner) ID() database.ID {
	return compoundId{id: s.StatefulSetUuid.String() + s.OwnerUuid.String()}
}

func NewStatefulSet() Resource {
	return &StatefulSet{}
}

func (s *StatefulSet) Obtain(k8s kmetav1.Object, clusterUuid types.UUID) {
	s.ObtainMeta(k8s, clusterUuid)

	statefulSet := k8s.(*kappsv1.StatefulSet)

	var replicas, ordinals int32
	if statefulSet.Spec.Replicas != nil {
		replicas = *statefulSet.Spec.Replicas
	}
	if statefulSet.Spec.Ordinals != nil {
		ordinals = statefulSet.Spec.Ordinals.Start
	}
	var pvcRetentionPolicyDeleted, pvcRetentionPolicyScaled kappsv1.PersistentVolumeClaimRetentionPolicyType
	if statefulSet.Spec.PersistentVolumeClaimRetentionPolicy != nil {
		pvcRetentionPolicyDeleted = statefulSet.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted
		pvcRetentionPolicyScaled = statefulSet.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled
	} else {
		pvcRetentionPolicyDeleted, pvcRetentionPolicyScaled = kappsv1.RetainPersistentVolumeClaimRetentionPolicyType, kappsv1.RetainPersistentVolumeClaimRetentionPolicyType
	}

	s.DesiredReplicas = replicas
	s.ServiceName = statefulSet.Spec.ServiceName
	s.PodManagementPolicy = string(statefulSet.Spec.PodManagementPolicy)
	s.UpdateStrategy = string(statefulSet.Spec.UpdateStrategy.Type)
	s.MinReadySeconds = statefulSet.Spec.MinReadySeconds
	s.PersistentVolumeClaimRetentionPolicyWhenDeleted = string(pvcRetentionPolicyDeleted)
	s.PersistentVolumeClaimRetentionPolicyWhenScaled = string(pvcRetentionPolicyScaled)
	s.Ordinals = ordinals
	s.ActualReplicas = statefulSet.Status.Replicas

	s.ReadyReplicas = statefulSet.Status.ReadyReplicas
	s.CurrentReplicas = statefulSet.Status.CurrentReplicas
	s.UpdatedReplicas = statefulSet.Status.UpdatedReplicas
	s.AvailableReplicas = statefulSet.Status.AvailableReplicas
	s.IcingaState, s.IcingaStateReason = s.getIcingaState()

	for _, condition := range statefulSet.Status.Conditions {
		s.Conditions = append(s.Conditions, StatefulSetCondition{
			StatefulSetUuid: s.Uuid,
			Type:            string(condition.Type),
			Status:          string(condition.Status),
			LastTransition:  types.UnixMilli(condition.LastTransitionTime.Time),
			Reason:          condition.Reason,
			Message:         condition.Message,
		})
	}

	for _, ownerReference := range statefulSet.OwnerReferences {
		var blockOwnerDeletion, controller bool
		if ownerReference.BlockOwnerDeletion != nil {
			blockOwnerDeletion = *ownerReference.BlockOwnerDeletion
		}
		if ownerReference.Controller != nil {
			controller = *ownerReference.Controller
		}
		s.Owners = append(s.Owners, StatefulSetOwner{
			StatefulSetUuid: s.Uuid,
			OwnerUuid:       EnsureUUID(ownerReference.UID),
			Kind:            strcase.Snake(ownerReference.Kind),
			Name:            ownerReference.Name,
			Uid:             ownerReference.UID,
			BlockOwnerDeletion: types.Bool{
				Bool:  blockOwnerDeletion,
				Valid: true,
			},
			Controller: types.Bool{
				Bool:  controller,
				Valid: true,
			},
		})
	}

	for labelName, labelValue := range statefulSet.Labels {
		labelUuid := NewUUID(s.Uuid, strings.ToLower(labelName+":"+labelValue))
		s.Labels = append(s.Labels, Label{
			Uuid:         labelUuid,
			ResourceUuid: s.Uuid,
			Name:         labelName,
			Value:        labelValue,
		})
		s.ResourceLabels = append(s.ResourceLabels, ResourceLabel{
			ResourceUuid: s.Uuid,
			LabelUuid:    labelUuid,
		})
	}

	for annotationName, annotationValue := range statefulSet.Annotations {
		annotationUuid := NewUUID(s.Uuid, strings.ToLower(annotationName+":"+annotationValue))
		s.Annotations = append(s.Annotations, Annotation{
			Uuid:         annotationUuid,
			ResourceUuid: s.Uuid,
			Name:         annotationName,
			Value:        annotationValue,
		})
		s.ResourceAnnotations = append(s.ResourceAnnotations, ResourceAnnotation{
			ResourceUuid:   s.Uuid,
			AnnotationUuid: annotationUuid,
		})
	}

	scheme := kruntime.NewScheme()
	_ = kappsv1.AddToScheme(scheme)
	codec := kserializer.NewCodecFactory(scheme).EncoderForVersion(kjson.NewYAMLSerializer(kjson.DefaultMetaFactory, scheme, scheme), kappsv1.SchemeGroupVersion)
	output, _ := kruntime.Encode(codec, statefulSet)
	s.Yaml = string(output)
}

func (s *StatefulSet) MarshalEvent() (notifications.Event, error) {
	return notifications.Event{
		Uuid:        s.Uuid,
		ClusterUuid: s.ClusterUuid,
		Kind:        "stateful_set",
		Name:        s.Namespace + "/" + s.Name,
		Severity:    s.IcingaState.ToSeverity(),
		Message:     s.IcingaStateReason,
		URL:         &url.URL{Path: "/statefulset", RawQuery: fmt.Sprintf("id=%s", s.Uuid)},
		Tags: map[string]string{
			"uuid":         s.Uuid.String(),
			"cluster_uuid": s.ClusterUuid.String(),
			"name":         s.Name,
			"namespace":    s.Namespace,
			"resource":     "stateful_set",
		},
	}, nil
}

func (s *StatefulSet) getIcingaState() (IcingaState, string) {
	switch {
	case s.AvailableReplicas == 0:
		reason := fmt.Sprintf("StatefulSet %s/%s has no replica available from %d desired.", s.Namespace, s.Name, s.DesiredReplicas)

		return Critical, reason
	case s.AvailableReplicas < s.DesiredReplicas:
		reason := fmt.Sprintf("StatefulSet %s/%s only has %d out of %d desired replicas available.", s.Namespace, s.Name, s.AvailableReplicas, s.DesiredReplicas)

		return Warning, reason
	default:
		reason := fmt.Sprintf("StatefulSet %s/%s has all %d desired replicas available.", s.Namespace, s.Name, s.DesiredReplicas)

		return Ok, reason
	}
}

func (s *StatefulSet) Relations() []kdatabase.Relation {
	fk := kdatabase.WithForeignKey("stateful_set_uuid")

	return []kdatabase.Relation{
		kdatabase.HasMany(s.Conditions, fk),
		kdatabase.HasMany(s.Owners, fk),
		kdatabase.HasMany(s.ResourceLabels, kdatabase.WithForeignKey("resource_uuid")),
		kdatabase.HasMany(s.Labels, kdatabase.WithForeignKey("resource_uuid")),
		kdatabase.HasMany(s.ResourceAnnotations, kdatabase.WithForeignKey("resource_uuid")),
		kdatabase.HasMany(s.Annotations, kdatabase.WithForeignKey("resource_uuid")),
		kdatabase.HasMany(s.Favorites, kdatabase.WithForeignKey("resource_uuid")),
	}
}
