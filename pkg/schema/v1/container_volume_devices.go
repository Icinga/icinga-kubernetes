package v1

type ContainerVolumeDevices struct {
	Namespace  string `db:"namespace"`
	PodName    string `db:"pod_name"`
	DeviceName string `db:"device_name"`
	DevicePath string `db:"device_path"`
}
