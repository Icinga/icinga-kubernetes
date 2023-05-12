package v1

type PodPvc struct {
	Namespace string `db:"namespace"`
	PodName   string `db:"pod_name"`
	ClaimName string `db:"claim_name"`
	ReadOnly  bool   `db:"read_only"`
}
