package schema

type PrometheusClusterMetric struct {
	kmetaWithoutNamespace
	Timestamp int64
	Group     string
	Name      string
	Value     float64
}

type PrometheusNodeMetric struct {
	kmetaWithoutNamespace
	NodeId    []byte
	Timestamp int64
	Group     string
	Name      string
	Value     float64
}

type PrometheusPodMetric struct {
	kmetaWithoutNamespace
	PodId     []byte
	Timestamp int64
	Group     string
	Name      string
	Value     float64
}

type PrometheusContainerMetric struct {
	kmetaWithoutNamespace
	ContainerId []byte
	Timestamp   int64
	Group       string
	Name        string
	Value       float64
}
