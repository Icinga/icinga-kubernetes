package schema

import "github.com/icinga/icinga-kubernetes/pkg/contracts"

type ContainerMetric struct {
	kmetaWithoutNamespace
	ContainerReferenceId []byte
	PodReferenceId       []byte
	Timestamp            int64
	Cpu                  int64
	Memory               int64
	Storage              int64
}

func NewContainerMetric(containerReferenceId []byte, podReferenceId []byte, timestamp int64, cpu int64, memory int64, storage int64) contracts.Resource {
	return &ContainerMetric{
		ContainerReferenceId: containerReferenceId,
		PodReferenceId:       podReferenceId,
		Timestamp:            timestamp,
		Cpu:                  cpu,
		Memory:               memory,
		Storage:              storage,
	}
}

type PodMetric struct {
	kmetaWithoutNamespace
	ReferenceId []byte
	Timestamp   int64
	Cpu         int64
	Memory      int64
	Storage     int64
}

func NewPodMetric(referenceId []byte, timestamp int64, cpu int64, memory int64, storage int64) *PodMetric {
	return &PodMetric{
		ReferenceId: referenceId,
		Timestamp:   timestamp,
		Cpu:         cpu,
		Memory:      memory,
		Storage:     storage,
	}
}

func (pm *PodMetric) IncreaseCpu(value int64) {
	pm.Cpu += value
}

func (pm *PodMetric) IncreaseMemory(value int64) {
	pm.Memory += value
}

func (pm *PodMetric) IncreaseStorage(value int64) {
	pm.Storage += value
}
