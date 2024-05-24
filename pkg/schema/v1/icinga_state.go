package v1

import (
	"database/sql/driver"
	"fmt"
)

type IcingaState uint8

const (
	Ok IcingaState = iota
	Warning
	Critical
	Unknown
	Pending IcingaState = 99
)

func (s IcingaState) String() string {
	switch s {
	case Ok:
		return "ok"
	case Warning:
		return "warning"
	case Critical:
		return "critical"
	case Unknown:
		return "unknown"
	case Pending:
		return "pending"
	default:
		panic(fmt.Sprintf("invalid Icinga state %d", s))
	}
}

// Value implements the driver.Valuer interface.
func (s IcingaState) Value() (driver.Value, error) {
	return s.String(), nil
}

// Assert interface compliance.
var (
	_ fmt.Stringer  = (*IcingaState)(nil)
	_ driver.Valuer = (*IcingaState)(nil)
)
