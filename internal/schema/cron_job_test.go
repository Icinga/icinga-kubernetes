package schema

import (
	"fmt"
	"testing"
	"time"

	kbatchv1 "k8s.io/api/batch/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestCronJobIcingaStateStartingDeadline(t *testing.T) {
	kathmandu, err := time.LoadLocation("Asia/Kathmandu")
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().Truncate(time.Minute)

	// daily returns a schedule that runs every day at the wall clock time of at in loc.
	daily := func(at time.Time, loc *time.Location) string {
		at = at.In(loc)

		return fmt.Sprintf("%d %d * * *", at.Minute(), at.Hour())
	}

	tests := []struct {
		name         string
		lastSchedule time.Time
		timezone     *string
		suspend      bool
		want         IcingaState
	}{
		{
			name:         "idle after success",
			lastSchedule: now.Add(-time.Hour),
			want:         Ok,
		},
		{
			// The schedule only matches the last schedule time in the configured time zone.
			name:         "idle after success with time zone",
			lastSchedule: now.Add(-20 * time.Hour),
			timezone:     new("Asia/Kathmandu"),
			want:         Ok,
		},
		{
			name:         "missed daily run",
			lastSchedule: now.Add(-25 * time.Hour),
			want:         Critical,
		},
		{
			name:         "suspended",
			lastSchedule: now.Add(-25 * time.Hour),
			suspend:      true,
			want:         Warning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc := time.UTC
			if tt.timezone != nil {
				loc = kathmandu
			}

			cronJob := &kbatchv1.CronJob{
				Namespace: "default",
				Name:      "report",
				UID:       "cron-job-uid",
				Spec: kbatchv1.CronJobSpec{
					Schedule:                   daily(tt.lastSchedule, loc),
					TimeZone:                   tt.timezone,
					StartingDeadlineSeconds:    ptr.To[int64](200),
					Suspend:                    new(tt.suspend),
					SuccessfulJobsHistoryLimit: ptr.To[int32](3),
					FailedJobsHistoryLimit:     ptr.To[int32](1),
				},
				Status: kbatchv1.CronJobStatus{
					LastScheduleTime:   new(kmetav1.NewTime(tt.lastSchedule)),
					LastSuccessfulTime: new(kmetav1.NewTime(tt.lastSchedule.Add(5 * time.Minute))),
				},
			}

			c := NewCronJob().(*CronJob)
			c.Obtain(cronJob, EnsureUUID("cluster-uid"))

			if c.IcingaState != tt.want {
				t.Errorf("IcingaState = %s, want %s (reason: %q)", c.IcingaState, tt.want, c.IcingaStateReason)
			}
		})
	}
}
