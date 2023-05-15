package v1

type VolumesInUse struct {
	Namespace  string `db:"namespace"`
	NodeName   string `db:"node_name"`
	VolumeName string `db:"volume_name"`
}
