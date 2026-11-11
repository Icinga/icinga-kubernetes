package schema

import (
	"database/sql/driver"
	"fmt"

	"github.com/icinga/icinga-go-library/notifications/event"
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

// ToSeverity converts the Icinga state to the corresponding Icinga Notifications severity.
func (s IcingaState) ToSeverity() event.Severity {
	switch s {
	case Ok, Pending:
		return event.SeverityOK
	case Unknown:
		return event.SeverityError
	case Warning:
		return event.SeverityWarning
	case Critical:
		return event.SeverityCritical
	default:
		panic(fmt.Sprintf("invalid Icinga state %d", s))
	}
}

// Assert interface compliance.
var (
	_ fmt.Stringer  = (*IcingaState)(nil)
	_ driver.Valuer = (*IcingaState)(nil)
)
