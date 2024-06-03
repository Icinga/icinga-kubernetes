package v1

import (
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	kbatchv1 "k8s.io/api/batch/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	kserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"strings"
)

type CronJob struct {
	Meta
	Schedule                   string
	Timezone                   string
	StartingDeadlineSeconds    int64
	ConcurrencyPolicy          string
	Suspend                    types.Bool
	SuccessfulJobsHistoryLimit int32
	FailedJobsHistoryLimit     int32
	Active                     int32
	LastScheduleTime           types.UnixMilli
	LastSuccessfulTime         types.UnixMilli
	Yaml                       string
	Labels                     []Label        `db:"-"`
	CronJobLabels              []CronJobLabel `db:"-"`
}

type CronJobLabel struct {
	CronJobUuid types.UUID
	LabelUuid   types.UUID
}

func NewCronJob() Resource {
	return &CronJob{}
}

func (c *CronJob) Obtain(k8s kmetav1.Object) {
	c.ObtainMeta(k8s)

	cronJob := k8s.(*kbatchv1.CronJob)

	var timeZone string
	if cronJob.Spec.TimeZone != nil {
		timeZone = *cronJob.Spec.TimeZone
	}
	var startingDeadlineSeconds int64
	if cronJob.Spec.StartingDeadlineSeconds != nil {
		startingDeadlineSeconds = *cronJob.Spec.StartingDeadlineSeconds
	}
	var suspend types.Bool
	if cronJob.Spec.Suspend != nil {
		suspend.Bool = *cronJob.Spec.Suspend
		suspend.Valid = true
	}
	var successfulJobsHistoryLimit int32
	if cronJob.Spec.SuccessfulJobsHistoryLimit != nil {
		successfulJobsHistoryLimit = *cronJob.Spec.SuccessfulJobsHistoryLimit
	}
	var failedJobsHistoryLimit int32
	if cronJob.Spec.FailedJobsHistoryLimit != nil {
		failedJobsHistoryLimit = *cronJob.Spec.FailedJobsHistoryLimit
	}
	if cronJob.Status.LastScheduleTime != nil {
		c.LastScheduleTime = types.UnixMilli(cronJob.Status.LastScheduleTime.Time)
	}
	if cronJob.Status.LastSuccessfulTime != nil {
		c.LastSuccessfulTime = types.UnixMilli(cronJob.Status.LastSuccessfulTime.Time)
	}

	c.Schedule = cronJob.Spec.Schedule
	c.Timezone = timeZone
	c.StartingDeadlineSeconds = startingDeadlineSeconds
	c.ConcurrencyPolicy = string(cronJob.Spec.ConcurrencyPolicy)
	c.Suspend = suspend
	c.SuccessfulJobsHistoryLimit = successfulJobsHistoryLimit
	c.FailedJobsHistoryLimit = failedJobsHistoryLimit
	c.Active = int32(len(cronJob.Status.Active))

	for labelName, labelValue := range cronJob.Labels {
		labelUuid := NewUUID(c.Uuid, strings.ToLower(labelName+":"+labelValue))
		c.Labels = append(c.Labels, Label{
			Uuid:  labelUuid,
			Name:  labelName,
			Value: labelValue,
		})
		c.CronJobLabels = append(c.CronJobLabels, CronJobLabel{
			CronJobUuid: c.Uuid,
			LabelUuid:   labelUuid,
		})
	}

	scheme := kruntime.NewScheme()
	_ = kbatchv1.AddToScheme(scheme)
	codec := kserializer.NewCodecFactory(scheme).EncoderForVersion(kjson.NewYAMLSerializer(kjson.DefaultMetaFactory, scheme, scheme), kbatchv1.SchemeGroupVersion)
	output, _ := kruntime.Encode(codec, cronJob)
	c.Yaml = string(output)
}

func (c *CronJob) Relations() []database.Relation {
	fk := database.WithForeignKey("cron_job_uuid")

	return []database.Relation{
		database.HasMany(c.Labels, database.WithoutCascadeDelete()),
		database.HasMany(c.CronJobLabels, fk),
	}
}
