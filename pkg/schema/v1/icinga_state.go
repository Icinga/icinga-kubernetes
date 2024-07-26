package v1

import (
	"database/sql/driver"
	"fmt"
)

type IcingaState uint8

const (
	Ok IcingaState = iota
	Pending
	Unknown
	Warning
	Critical
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

func (s IcingaState) ToSeverity() string {
	switch s {
	case Ok:
		return "ok"
	case Pending:
		return "info"
	case Unknown:
		return "err"
	case Warning:
		return "warning"
	case Critical:
		return "crit"
	default:
		panic(fmt.Sprintf("invalid Icinga state %d", s))
	}
}

// Assert interface compliance.
var (
	_ fmt.Stringer  = (*IcingaState)(nil)
	_ driver.Valuer = (*IcingaState)(nil)
)
