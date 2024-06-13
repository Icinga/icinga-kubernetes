package v1

import (
	"database/sql/driver"
	"github.com/icinga/icinga-kubernetes/pkg/strcase"
	kcorev1 "k8s.io/api/core/v1"
)

type ImagePullPolicy kcorev1.PullPolicy

// Value implements the [driver.Valuer] interface.
func (p ImagePullPolicy) Value() (driver.Value, error) {
	return strcase.Snake(string(p)), nil
}

// Assert interface compliance.
var (
	_ driver.Valuer = (*IcingaState)(nil)
)
