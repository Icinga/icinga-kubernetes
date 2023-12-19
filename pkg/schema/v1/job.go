package v1

import (
	"database/sql"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/strcase"
	"github.com/icinga/icinga-kubernetes/pkg/types"
	kbatchv1 "k8s.io/api/batch/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

type Job struct {
	Meta
	Id                      types.Binary
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
	Conditions              []JobCondition `db:"-"`
	Labels                  []Label        `db:"-"`
	JobLabels               []JobLabel     `db:"-"`
}

type JobCondition struct {
	JobId          types.Binary
	Type           string
	Status         string
	LastProbe      types.UnixMilli
	LastTransition types.UnixMilli
	Reason         string
	Message        string
}

type JobLabel struct {
	JobId   types.Binary
	LabelId types.Binary
}

func NewJob() Resource {
	return &Job{}
}

func (j *Job) Obtain(k8s kmetav1.Object) {
	j.ObtainMeta(k8s)

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
		completionMode.String = strcase.Snake(string(*job.Spec.CompletionMode))
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

	j.Id = types.Checksum(j.Namespace + "/" + j.Name)
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

	for _, condition := range job.Status.Conditions {
		j.Conditions = append(j.Conditions, JobCondition{
			JobId:          j.Id,
			Type:           strcase.Snake(string(condition.Type)),
			Status:         strcase.Snake(string(condition.Status)),
			LastProbe:      types.UnixMilli(condition.LastProbeTime.Time),
			LastTransition: types.UnixMilli(condition.LastTransitionTime.Time),
			Reason:         condition.Reason,
			Message:        condition.Message,
		})
	}

	for labelName, labelValue := range job.Labels {
		labelId := types.Checksum(strings.ToLower(labelName + ":" + labelValue))
		j.Labels = append(j.Labels, Label{
			Id:    labelId,
			Name:  labelName,
			Value: labelValue,
		})
		j.JobLabels = append(j.JobLabels, JobLabel{
			JobId:   j.Id,
			LabelId: labelId,
		})
	}
}

func (j *Job) Relations() []database.Relation {
	fk := database.WithForeignKey("job_id")

	return []database.Relation{
		database.HasMany(j.Conditions, fk),
		database.HasMany(j.Labels, database.WithoutCascadeDelete()),
		database.HasMany(j.JobLabels, fk),
	}
}
