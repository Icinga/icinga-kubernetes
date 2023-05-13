package v1

type VolumesAttached struct {
	Namespace  string `db:"namespace"`
	NodeName   string `db:"node_name"`
	VolumeName string `db:"volume_name"`
	DevicePath string `db:"device_path"`
}
