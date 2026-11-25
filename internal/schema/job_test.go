package schema

import (
	"testing"

	kbatchv1 "k8s.io/api/batch/v1"
	kcorev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func TestJobIcingaState(t *testing.T) {
	failureTarget := kbatchv1.JobCondition{
		Type:    kbatchv1.JobFailureTarget,
		Status:  kcorev1.ConditionTrue,
		Reason:  kbatchv1.JobReasonPodFailurePolicy,
		Message: "Container main for pod default/backup-x7k2p failed with exit code 42 matching FailJob rule at index 0",
	}
	failed := kbatchv1.JobCondition{
		Type:    kbatchv1.JobFailed,
		Status:  kcorev1.ConditionTrue,
		Reason:  kbatchv1.JobReasonPodFailurePolicy,
		Message: "Container main for pod default/backup-x7k2p failed with exit code 42 matching FailJob rule at index 0",
	}
	successCriteriaMet := kbatchv1.JobCondition{
		Type:   kbatchv1.JobSuccessCriteriaMet,
		Status: kcorev1.ConditionTrue,
		Reason: kbatchv1.JobReasonCompletionsReached,
	}
	complete := kbatchv1.JobCondition{
		Type:   kbatchv1.JobComplete,
		Status: kcorev1.ConditionTrue,
		Reason: kbatchv1.JobReasonCompletionsReached,
	}

	tests := []struct {
		name       string
		conditions []kbatchv1.JobCondition
		want       IcingaState
	}{
		{"running", nil, Pending},
		{"failure target only", []kbatchv1.JobCondition{failureTarget}, Warning},
		// The Job controller adds FailureTarget first and keeps it after adding Failed.
		{"failed after failure target", []kbatchv1.JobCondition{failureTarget, failed}, Critical},
		{"failed", []kbatchv1.JobCondition{failed}, Critical},
		{"complete after success criteria met", []kbatchv1.JobCondition{successCriteriaMet, complete}, Ok},
		{
			"failure target not true",
			[]kbatchv1.JobCondition{{Type: kbatchv1.JobFailureTarget, Status: kcorev1.ConditionFalse}, failed},
			Critical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &kbatchv1.Job{
				Namespace: "default",
				Name:      "backup",
				UID:       "job-uid",
				Spec:      kbatchv1.JobSpec{BackoffLimit: ptr.To[int32](6)},
				Status:    kbatchv1.JobStatus{Conditions: tt.conditions},
			}

			j := NewJob().(*Job)
			j.Obtain(job, EnsureUUID("cluster-uid"))

			if j.IcingaState != tt.want {
				t.Errorf("IcingaState = %s, want %s (reason: %q)", j.IcingaState, tt.want, j.IcingaStateReason)
			}
		})
	}
}
