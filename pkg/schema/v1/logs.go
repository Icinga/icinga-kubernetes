package v1

type ContainerLog struct {
	ContainerName string `db:"container_name"`
	PodName       string `db:"pod_name"`
	Namespace     string
	Logs          string
}
