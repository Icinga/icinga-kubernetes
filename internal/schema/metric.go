package schema

import (
	"strconv"

	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/internal/kdatabase"
)

type PrometheusClusterMetric struct {
	kdatabase.EntityStub
	ClusterUuid types.UUID
	Timestamp   int64
	Category    string
	Name        string
	Value       float64
}

func (m PrometheusClusterMetric) ID() database.ID {
	return compoundId{id: m.ClusterUuid.String() + strconv.FormatInt(m.Timestamp, 10) + m.Category + m.Name}
}

type PrometheusNodeMetric struct {
	kdatabase.EntityStub
	NodeUuid  types.UUID
	Timestamp int64
	Category  string
	Name      string
	Value     float64
}

func (m PrometheusNodeMetric) ID() database.ID {
	return compoundId{id: m.NodeUuid.String() + strconv.FormatInt(m.Timestamp, 10) + m.Category + m.Name}
}

type PrometheusPodMetric struct {
	kdatabase.EntityStub
	PodUuid   types.UUID
	Timestamp int64
	Category  string
	Name      string
	Value     float64
}

func (m PrometheusPodMetric) ID() database.ID {
	return compoundId{id: m.PodUuid.String() + strconv.FormatInt(m.Timestamp, 10) + m.Category + m.Name}
}

type PrometheusContainerMetric struct {
	kdatabase.EntityStub
	ContainerUuid types.UUID
	Timestamp     int64
	Category      string
	Name          string
	Value         float64
}

func (m PrometheusContainerMetric) ID() database.ID {
	return compoundId{id: m.ContainerUuid.String() + strconv.FormatInt(m.Timestamp, 10) + m.Category + m.Name}
}
