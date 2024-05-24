package v1

import (
	"github.com/icinga/icinga-go-library/types"
	"time"
)

type PodMetrics struct {
	Namespace             string          `db:"namespace"`
	PodName               string          `db:"pod_name"`
	ContainerName         string          `db:"container_name"`
	Timestamp             types.UnixMilli `db:"timestamp"`
	Duration              time.Duration   `db:"duration"`
	CPUUsage              float64         `db:"cpu_usage"`
	MemoryUsage           float64         `db:"memory_usage"`
	StorageUsage          float64         `db:"storage_usage"`
	EphemeralStorageUsage float64         `db:"ephemeral_storage_usage"`
}
