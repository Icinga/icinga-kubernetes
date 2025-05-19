package v1

import (
	"database/sql"
	"fmt"
	"github.com/icinga/icinga-go-library/strcase"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	kbatchv1 "k8s.io/api/batch/v1"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	kserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	ktypes "k8s.io/apimachinery/pkg/types"
	"strings"
)

type Job struct {
	Meta
	Parallelism             sql.NullInt32
	Completions             sql.NullInt32
	ActiveDeadlineSeconds   sql.NullInt64
	BackoffLimit            sql.NullInt32
	TtlSecondsAfterFinished sql.NullInt32
	CompletionMode          sql.NullString
	Suspend                 types.Bool
	StartTime               types.UnixMilli
	CompletionTime          types.UnixMilli
	Active                  int32
	Succeeded               int32
	Failed                  int32
	Yaml                    string
	IcingaState             IcingaState
	IcingaStateReason       string
	Conditions              []JobCondition       `db:"-"`
	Labels                  []Label              `db:"-"`
	JobLabels               []JobLabel           `db:"-"`
	ResourceLabels          []ResourceLabel      `db:"-"`
	Annotations             []Annotation         `db:"-"`
	JobAnnotations          []JobAnnotation      `db:"-"`
	ResourceAnnotations     []ResourceAnnotation `db:"-"`
	Owners                  []JobOwner           `db:"-"`
	Favorites               []Favorite           `db:"-"`
}

type JobCondition struct {
	JobUuid        types.UUID
	Type           string
	Status         string
	LastProbe      types.UnixMilli
	LastTransition types.UnixMilli
	Reason         string
	Message        string
}

type JobLabel struct {
	JobUuid   types.UUID
	LabelUuid types.UUID
}

type JobAnnotation struct {
	JobUuid        types.UUID
	AnnotationUuid types.UUID
}

type JobOwner struct {
	JobUuid            types.UUID
	OwnerUuid          types.UUID
	Kind               string
	Name               string
	Uid                ktypes.UID
	Controller         types.Bool
	BlockOwnerDeletion types.Bool
}

func NewJob() Resource {
	return &Job{}
}

func (j *Job) Obtain(k8s kmetav1.Object, clusterUuid types.UUID) {
	j.ObtainMeta(k8s, clusterUuid)

	job := k8s.(*kbatchv1.Job)

	var parallelism sql.NullInt32
	if job.Spec.Parallelism != nil {
		parallelism.Int32 = *job.Spec.Parallelism
		parallelism.Valid = true
	}
	var completions sql.NullInt32
	if job.Spec.Completions != nil {
		completions.Int32 = *job.Spec.Completions
		completions.Valid = true
	}
	var activeDeadlineSeconds sql.NullInt64
	if job.Spec.ActiveDeadlineSeconds != nil {
		activeDeadlineSeconds.Int64 = *job.Spec.ActiveDeadlineSeconds
		activeDeadlineSeconds.Valid = true
	}
	var backoffLimit sql.NullInt32
	if job.Spec.BackoffLimit != nil {
		backoffLimit.Int32 = *job.Spec.BackoffLimit
		backoffLimit.Valid = true
	}
	var ttlSecondsAfterFinished sql.NullInt32
	if job.Spec.TTLSecondsAfterFinished != nil {
		ttlSecondsAfterFinished.Int32 = *job.Spec.TTLSecondsAfterFinished
		ttlSecondsAfterFinished.Valid = true
	}
	var suspend types.Bool
	if job.Spec.Suspend != nil {
		suspend.Bool = *job.Spec.Suspend
		suspend.Valid = true
	}
	var completionMode sql.NullString
	if job.Spec.CompletionMode != nil {
		completionMode.String = string(*job.Spec.CompletionMode)
		completionMode.Valid = true
	}
	var startTime kmetav1.Time
	if job.Status.StartTime != nil {
		startTime = *job.Status.StartTime
	}
	var completionTime kmetav1.Time
	if job.Status.CompletionTime != nil {
		completionTime = *job.Status.CompletionTime
	}

	j.Parallelism = parallelism
	j.Completions = completions
	j.ActiveDeadlineSeconds = activeDeadlineSeconds
	j.BackoffLimit = backoffLimit
	j.TtlSecondsAfterFinished = ttlSecondsAfterFinished
	j.Suspend = suspend
	j.CompletionMode = completionMode
	j.StartTime = types.UnixMilli(startTime.Time)
	j.CompletionTime = types.UnixMilli(completionTime.Time)
	j.Active = job.Status.Active
	j.Succeeded = job.Status.Succeeded
	j.Failed = job.Status.Failed
	j.IcingaState, j.IcingaStateReason = j.getIcingaState(job)

	for _, condition := range job.Status.Conditions {
		j.Conditions = append(j.Conditions, JobCondition{
			JobUuid:        j.Uuid,
			Type:           string(condition.Type),
			Status:         string(condition.Status),
			LastProbe:      types.UnixMilli(condition.LastProbeTime.Time),
			LastTransition: types.UnixMilli(condition.LastTransitionTime.Time),
			Reason:         condition.Reason,
			Message:        condition.Message,
		})
	}

	for labelName, labelValue := range job.Labels {
		labelUuid := NewUUID(j.Uuid, strings.ToLower(labelName+":"+labelValue))
		j.Labels = append(j.Labels, Label{
			Uuid:  labelUuid,
			Name:  labelName,
			Value: labelValue,
		})
		j.JobLabels = append(j.JobLabels, JobLabel{
			JobUuid:   j.Uuid,
			LabelUuid: labelUuid,
		})
		j.ResourceLabels = append(j.ResourceLabels, ResourceLabel{
			ResourceUuid: j.Uuid,
			LabelUuid:    labelUuid,
		})
	}

	for annotationName, annotationValue := range job.Annotations {
		annotationUuid := NewUUID(j.Uuid, strings.ToLower(annotationName+":"+annotationValue))
		j.Annotations = append(j.Annotations, Annotation{
			Uuid:  annotationUuid,
			Name:  annotationName,
			Value: annotationValue,
		})
		j.JobAnnotations = append(j.JobAnnotations, JobAnnotation{
			JobUuid:        j.Uuid,
			AnnotationUuid: annotationUuid,
		})
		j.ResourceAnnotations = append(j.ResourceAnnotations, ResourceAnnotation{
			ResourceUuid:   j.Uuid,
			AnnotationUuid: annotationUuid,
		})
	}

	for _, ownerReference := range job.OwnerReferences {
		var blockOwnerDeletion, controller bool
		if ownerReference.BlockOwnerDeletion != nil {
			blockOwnerDeletion = *ownerReference.BlockOwnerDeletion
		}
		if ownerReference.Controller != nil {
			controller = *ownerReference.Controller
		}
		j.Owners = append(j.Owners, JobOwner{
			JobUuid:   j.Uuid,
			OwnerUuid: EnsureUUID(ownerReference.UID),
			Kind:      strcase.Snake(ownerReference.Kind),
			Name:      ownerReference.Name,
			Uid:       ownerReference.UID,
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

	scheme := kruntime.NewScheme()
	_ = kbatchv1.AddToScheme(scheme)
	codec := kserializer.NewCodecFactory(scheme).EncoderForVersion(kjson.NewYAMLSerializer(kjson.DefaultMetaFactory, scheme, scheme), kbatchv1.SchemeGroupVersion)
	output, _ := kruntime.Encode(codec, job)
	j.Yaml = string(output)
}

func (j *Job) getIcingaState(job *kbatchv1.Job) (IcingaState, string) {
	for _, condition := range job.Status.Conditions {
		if condition.Status != kcorev1.ConditionTrue {
			continue
		}

		switch condition.Type {
		case kbatchv1.JobSuccessCriteriaMet:
			return Ok, fmt.Sprintf(
				"Job %s/%s met its sucess criteria.",
				j.Namespace, j.Name)
		case kbatchv1.JobComplete:
			reason := fmt.Sprintf(
				"Job %s/%s has completed its execution successfully with",
				j.Namespace, j.Name)

			if j.Completions.Valid {
				reason += fmt.Sprintf(" %d necessary pod completions.", j.Completions.Int32)
			} else {
				reason += " any pod completion."
			}

			return Ok, reason
		case kbatchv1.JobFailed:
			return Critical, fmt.Sprintf(
				"Job %s/%s has failed its execution. %s: %s.",
				j.Namespace, j.Name, condition.Reason, condition.Message)
		case kbatchv1.JobFailureTarget:
			return Warning, fmt.Sprintf(
				"Job %s/%s is about to fail its execution. %s: %s.",
				j.Namespace, j.Name, condition.Reason, condition.Message)
		case kbatchv1.JobSuspended:
			return Ok, fmt.Sprintf(
				"Job %s/%s is suspended.",
				j.Namespace, j.Name)
		}
	}

	var completions string
	if j.Completions.Valid {
		completions = fmt.Sprintf("%d pod completions", j.Completions.Int32)
	} else {
		completions = "any pod completion"
	}

	reason := fmt.Sprintf(
		"Job %s/%s is running since %s with currently %d active, %d completed and %d failed pods. "+
			"Successful termination requires %s. The back-off limit is %d.",
		j.Namespace, j.Name, job.Status.StartTime, j.Active, j.Succeeded, j.Failed, completions, *job.Spec.BackoffLimit)

	if job.Spec.ActiveDeadlineSeconds != nil {
		reason += fmt.Sprintf(" Deadline for completion is %d.", job.Spec.ActiveDeadlineSeconds)
	}

	return Pending, reason
}

func (j *Job) Relations() []database.Relation {
	fk := database.WithForeignKey("job_uuid")

	return []database.Relation{
		database.HasMany(j.Conditions, fk),
		database.HasMany(j.ResourceLabels, database.WithForeignKey("resource_uuid")),
		database.HasMany(j.Labels, database.WithoutCascadeDelete()),
		database.HasMany(j.JobLabels, fk),
		database.HasMany(j.ResourceAnnotations, database.WithForeignKey("resource_uuid")),
		database.HasMany(j.Annotations, database.WithoutCascadeDelete()),
		database.HasMany(j.JobAnnotations, fk),
		database.HasMany(j.Owners, fk),
		database.HasMany(j.Favorites, database.WithForeignKey("resource_uuid")),
	}
}
