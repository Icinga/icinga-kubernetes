package schema

type PodMetric struct {
	kmetaWithoutNamespace
	ReferenceId []byte
	Timestamp   int64
	Cpu         int64
	Memory      int64
	Storage     int64
}

type ContainerMetric struct {
	kmetaWithoutNamespace
	ContainerReferenceId []byte
	PodReferenceId       []byte
	Timestamp            int64
	Cpu                  int64
	Memory               int64
	Storage              int64
}

type NodeMetric struct {
	kmetaWithoutNamespace
	NodeId    []byte
	Timestamp int64
	Cpu       int64
	Memory    int64
	Storage   int64
}
