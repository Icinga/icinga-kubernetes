package v1

import (
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	"strconv"
)

type PrometheusClusterMetric struct {
	ClusterUuid types.UUID
	Timestamp   int64
	Category    string
	Name        string
	Value       float64
}

func (m *PrometheusClusterMetric) ID() database.ID {
	return compoundId{id: m.ClusterUuid.String() + strconv.FormatInt(m.Timestamp, 10) + m.Category + m.Name}
}

func (m *PrometheusClusterMetric) SetID(id database.ID) {
	panic("Not expected to be called")
}

func (m *PrometheusClusterMetric) Fingerprint() database.Fingerprinter {
	return m
}

type PrometheusNodeMetric struct {
	NodeUuid  types.UUID
	Timestamp int64
	Category  string
	Name      string
	Value     float64
}

func (m *PrometheusNodeMetric) ID() database.ID {
	return compoundId{id: m.NodeUuid.String() + strconv.FormatInt(m.Timestamp, 10) + m.Category + m.Name}
}

func (m *PrometheusNodeMetric) SetID(id database.ID) {
	panic("Not expected to be called")
}

func (m *PrometheusNodeMetric) Fingerprint() database.Fingerprinter {
	return m
}

type PrometheusPodMetric struct {
	PodUuid   types.UUID
	Timestamp int64
	Category  string
	Name      string
	Value     float64
}

func (m *PrometheusPodMetric) ID() database.ID {
	return compoundId{id: m.PodUuid.String() + strconv.FormatInt(m.Timestamp, 10) + m.Category + m.Name}
}

func (m *PrometheusPodMetric) SetID(id database.ID) {
	panic("Not expected to be called")
}

func (m *PrometheusPodMetric) Fingerprint() database.Fingerprinter {
	return m
}

type PrometheusContainerMetric struct {
	ContainerUuid types.UUID
	Timestamp     int64
	Category      string
	Name          string
	Value         float64
}

func (m *PrometheusContainerMetric) ID() database.ID {
	return compoundId{id: m.ContainerUuid.String() + strconv.FormatInt(m.Timestamp, 10) + m.Category + m.Name}
}

func (m *PrometheusContainerMetric) SetID(id database.ID) {
	panic("Not expected to be called")
}

func (m *PrometheusContainerMetric) Fingerprint() database.Fingerprinter {
	return m
}

type compoundId struct {
	id string
}

func (i compoundId) String() string {
	return i.id
}
