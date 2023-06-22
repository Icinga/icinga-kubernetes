package database

import (
	sqlDriver "database/sql/driver"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/icinga/icinga-kubernetes/pkg/strcase"
	"github.com/icinga/icinga-kubernetes/pkg/types"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"net"
	"strings"
)

// CantPerformQuery wraps the given error with the specified query that cannot be executed.
func CantPerformQuery(err error, q string) error {
	return errors.Wrapf(err, "can't perform %q", q)
}

func IsUnixAddr(host string) bool {
	return strings.HasPrefix(host, "/")
}

// JoinHostPort is like its equivalent in net., but handles UNIX sockets as well.
func JoinHostPort(host string, port int) string {
	if IsUnixAddr(host) {
		return host
	}

	return net.JoinHostPort(host, fmt.Sprint(port))
}

// IsRetryable checks whether the given error is retryable.
func IsRetryable(err error) bool {
	if errors.Is(err, sqlDriver.ErrBadConn) {
		return true
	}

	if errors.Is(err, mysql.ErrInvalidConn) {
		return true
	}

	var e *mysql.MySQLError
	if errors.As(err, &e) {
		switch e.Number {
		case 1053, 1205, 1213, 2006:
			// 1053: Server shutdown in progress
			// 1205: Lock wait timeout
			// 1213: Deadlock found when trying to get lock
			// 2006: MySQL server has gone away
			return true
		default:
			return false
		}
	}

	var pe *pq.Error
	if errors.As(err, &pe) {
		switch pe.Code {
		case "08000", // connection_exception
			"08006", // connection_failure
			"08001", // sqlclient_unable_to_establish_sqlconnection
			"08004", // sqlserver_rejected_establishment_of_sqlconnection
			"40001", // serialization_failure
			"40P01", // deadlock_detected
			"54000", // program_limit_exceeded
			"55006", // object_in_use
			"55P03", // lock_not_available
			"57P01", // admin_shutdown
			"57P02", // crash_shutdown
			"57P03", // cannot_connect_now
			"58000", // system_error
			"58030", // io_error
			"XX000": // internal_error
			return true
		default:
			if strings.HasPrefix(string(pe.Code), "53") {
				// Class 53 - Insufficient Resources
				return true
			}
		}
	}

	return false
}

// TableName returns the table of t.
func TableName(t interface{}) string {
	if tn, ok := t.(TableNamer); ok {
		return tn.TableName()
	}

	if s, ok := t.(string); ok {
		return s
	}

	return strcase.Snake(types.Name(t))
}
