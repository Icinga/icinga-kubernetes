package v1

type Volumes struct {
	Namespace    string
	PodName      string `db:"pod_name"`
	Name         string
	Type         string
	VolumeSource string `db:"volume_source"`
}
