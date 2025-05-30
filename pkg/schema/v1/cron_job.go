package v1

import (
	"database/sql"
	"fmt"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	kbatchv1 "k8s.io/api/batch/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	kserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"strings"
	"time"
)

type CronJob struct {
	Meta
	Schedule                   string
	Timezone                   sql.NullString
	StartingDeadlineSeconds    sql.NullInt64
	ConcurrencyPolicy          string
	Suspend                    types.Bool
	SuccessfulJobsHistoryLimit int32
	FailedJobsHistoryLimit     int32
	Active                     int32
	LastScheduleTime           types.UnixMilli
	LastSuccessfulTime         types.UnixMilli
	Yaml                       string
	IcingaState                IcingaState
	IcingaStateReason          string
	Labels                     []Label              `db:"-"`
	CronJobLabels              []CronJobLabel       `db:"-"`
	ResourceLabels             []ResourceLabel      `db:"-"`
	Annotations                []Annotation         `db:"-"`
	CronJobAnnotations         []CronJobAnnotation  `db:"-"`
	ResourceAnnotations        []ResourceAnnotation `db:"-"`
	Favorites                  []Favorite           `db:"-"`
}

type CronJobLabel struct {
	CronJobUuid types.UUID
	LabelUuid   types.UUID
}

type CronJobAnnotation struct {
	CronJobUuid    types.UUID
	AnnotationUuid types.UUID
}

func NewCronJob() Resource {
	return &CronJob{}
}

func (c *CronJob) Obtain(k8s kmetav1.Object, clusterUuid types.UUID) {
	c.ObtainMeta(k8s, clusterUuid)

	cronJob := k8s.(*kbatchv1.CronJob)

	c.Schedule = cronJob.Spec.Schedule
	c.Timezone = NewNullableString(cronJob.Spec.TimeZone)
	if cronJob.Spec.StartingDeadlineSeconds != nil {
		c.StartingDeadlineSeconds.Int64 = *cronJob.Spec.StartingDeadlineSeconds
		c.StartingDeadlineSeconds.Valid = true
	}
	c.ConcurrencyPolicy = string(cronJob.Spec.ConcurrencyPolicy)
	// It is safe to use the pointer directly here,
	// as Kubernetes sets it to false by default.
	c.Suspend.Bool = *cronJob.Spec.Suspend
	c.Suspend.Valid = true
	// It is safe to use the pointer directly here,
	// as Kubernetes sets it to 3 if not configured.
	c.SuccessfulJobsHistoryLimit = *cronJob.Spec.SuccessfulJobsHistoryLimit
	// It is safe to use the pointer directly here,
	// as Kubernetes sets it to 1 if not configured.
	c.FailedJobsHistoryLimit = *cronJob.Spec.FailedJobsHistoryLimit

	c.Active = int32(len(cronJob.Status.Active))
	if cronJob.Status.LastScheduleTime != nil {
		c.LastScheduleTime = types.UnixMilli(cronJob.Status.LastScheduleTime.Time)
	}
	if cronJob.Status.LastSuccessfulTime != nil {
		c.LastSuccessfulTime = types.UnixMilli(cronJob.Status.LastSuccessfulTime.Time)
	}

	c.IcingaState, c.IcingaStateReason = c.getIcingaState()

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
		c.ResourceLabels = append(c.ResourceLabels, ResourceLabel{
			ResourceUuid: c.Uuid,
			LabelUuid:    labelUuid,
		})
	}

	for annotationName, annotationValue := range cronJob.Annotations {
		annotationUuid := NewUUID(c.Uuid, strings.ToLower(annotationName+":"+annotationValue))
		c.Annotations = append(c.Annotations, Annotation{
			Uuid:  annotationUuid,
			Name:  annotationName,
			Value: annotationValue,
		})
		c.CronJobAnnotations = append(c.CronJobAnnotations, CronJobAnnotation{
			CronJobUuid:    c.Uuid,
			AnnotationUuid: annotationUuid,
		})
		c.ResourceAnnotations = append(c.ResourceAnnotations, ResourceAnnotation{
			ResourceUuid:   c.Uuid,
			AnnotationUuid: annotationUuid,
		})
	}

	scheme := kruntime.NewScheme()
	_ = kbatchv1.AddToScheme(scheme)
	codec := kserializer.NewCodecFactory(scheme).EncoderForVersion(kjson.NewYAMLSerializer(kjson.DefaultMetaFactory, scheme, scheme), kbatchv1.SchemeGroupVersion)
	output, _ := kruntime.Encode(codec, cronJob)
	c.Yaml = string(output)
}

func (c *CronJob) getIcingaState() (IcingaState, string) {
	now := time.Now()

	if c.LastScheduleTime.Time().IsZero() {
		return Warning, fmt.Sprintf("CronJob %s has never been scheduled.", c.Name)
	}

	if c.LastSuccessfulTime.Time().IsZero() {
		return Critical, fmt.Sprintf("CronJob %s has never completed successfully.", c.Name)
	}

	if c.StartingDeadlineSeconds.Valid {
		deadlineDuration := time.Duration(c.StartingDeadlineSeconds.Int64) * time.Second
		deadline := c.LastScheduleTime.Time().Add(deadlineDuration)

		if now.After(deadline) {
			return Critical, fmt.Sprintf("CronJob %s missed its starting deadline. Last scheduled at %v, deadline was %v.",
				c.Name, c.LastScheduleTime.Time().Format(time.RFC3339), deadline.Format(time.RFC3339))
		}
	}

	if c.LastScheduleTime.Time().After(c.LastSuccessfulTime.Time()) {
		return Warning, fmt.Sprintf("CronJob %s has recent schedules without success. Last successful run: %v, last scheduled: %v.",
			c.Name, c.LastSuccessfulTime.Time().Format(time.RFC3339), c.LastScheduleTime.Time().Format(time.RFC3339))
	}

	if c.Suspend.Valid && c.Suspend.Bool {
		return Warning, fmt.Sprintf("CronJob %s is currently suspended.", c.Name)
	}

	return Ok, fmt.Sprintf("CronJob %s is operating normally. Last successful run: %v.",
		c.Name, c.LastSuccessfulTime.Time().Format(time.RFC3339))
}

func (c *CronJob) Relations() []database.Relation {
	fk := database.WithForeignKey("cron_job_uuid")

	return []database.Relation{
		database.HasMany(c.ResourceLabels, database.WithForeignKey("resource_uuid")),
		database.HasMany(c.Labels, database.WithoutCascadeDelete()),
		database.HasMany(c.CronJobLabels, fk),
		database.HasMany(c.ResourceAnnotations, database.WithForeignKey("resource_uuid")),
		database.HasMany(c.Annotations, database.WithoutCascadeDelete()),
		database.HasMany(c.CronJobAnnotations, fk),
		database.HasMany(c.Favorites, database.WithForeignKey("resource_uuid")),
	}
}
