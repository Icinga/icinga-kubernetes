package database

import (
	"testing"
	"time"
)

func TestClassifyClusterRemovalState(t *testing.T) {
	now := time.Date(
		2026,
		time.September,
		16,
		10,
		0,
		0,
		0,
		time.UTC,
	)
	activeWithin := 5 * time.Minute

	tests := []struct {
		name            string
		instanceCount   int64
		latestHeartbeat time.Time
		want            ClusterRemovalState
	}{
		{
			name:            "no instance",
			instanceCount:   0,
			latestHeartbeat: time.Time{},
			want:            ClusterRemovalStateNoInstance,
		},
		{
			name:            "fresh heartbeat",
			instanceCount:   1,
			latestHeartbeat: now.Add(-30 * time.Second),
			want:            ClusterRemovalStateActive,
		},
		{
			name:            "heartbeat exactly on boundary",
			instanceCount:   1,
			latestHeartbeat: now.Add(-activeWithin),
			want:            ClusterRemovalStateActive,
		},
		{
			name:          "stale heartbeat",
			instanceCount: 1,
			latestHeartbeat: now.Add(
				-activeWithin - time.Millisecond,
			),
			want: ClusterRemovalStateStale,
		},
		{
			name:            "future heartbeat remains active",
			instanceCount:   1,
			latestHeartbeat: now.Add(time.Minute),
			want:            ClusterRemovalStateActive,
		},
		{
			name:            "multiple instances use newest heartbeat",
			instanceCount:   2,
			latestHeartbeat: now.Add(-time.Minute),
			want:            ClusterRemovalStateActive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyClusterRemovalState(
				tt.instanceCount,
				tt.latestHeartbeat,
				now,
				activeWithin,
			)

			if got != tt.want {
				t.Fatalf(
					"classifyClusterRemovalState() = %q, want %q",
					got,
					tt.want,
				)
			}
		})
	}
}
