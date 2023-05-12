package v1

type ContainerVolumeMount struct {
	Namespace string `db:"namespace"`
	PodName   string `db:"pod_name"`
	MountName string `db:"mount_name"`
	ReadOnly  bool   `db:"read_only"`
	MountPath string `db:"mount_path"`
	SubPath   string `db:"sub_path"`
}
