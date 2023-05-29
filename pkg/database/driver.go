package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/go-sql-driver/mysql"
	"github.com/icinga/icinga-kubernetes/pkg/backoff"
	"github.com/icinga/icinga-kubernetes/pkg/retry"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"time"
)

const MySQL = "icinga-mysql"
const PostgreSQL = "icinga-pgsql"

var timeout = time.Minute * 5

// RetryConnector wraps driver.Connector with retry logic.
type RetryConnector struct {
	driver.Connector
	driver Driver
}

// Connect implements part of the driver.Connector interface.
func (c RetryConnector) Connect(ctx context.Context) (driver.Conn, error) {
	var conn driver.Conn
	err := errors.Wrap(retry.WithBackoff(
		ctx,
		func(ctx context.Context) (err error) {
			conn, err = c.Connector.Connect(ctx)
			return
		},
		shouldRetry,
		backoff.NewExponentialWithJitter(time.Millisecond*128, time.Minute*1),
		retry.Settings{
			Timeout: timeout,
			OnError: func(_ time.Duration, _ uint64, err, lastErr error) {
				if lastErr == nil || err.Error() != lastErr.Error() {
					c.driver.Logger.Info("Can't connect to database. Retrying", "error", err)
				}
			},
			OnSuccess: func(elapsed time.Duration, attempt uint64, _ error) {
				if attempt > 0 {
					c.driver.Logger.Info("Reconnected to database")
					// c.driver.Logger.Info(1, "Reconnected to database",
					// 	zap.Duration("after", elapsed), zap.Uint64("attempts", attempt+1))
				}
			},
		},
	), "can't connect to database")
	return conn, err
}

// Driver implements part of the driver.Connector interface.
func (c RetryConnector) Driver() driver.Driver {
	return c.driver
}

// Driver wraps a driver.Driver that also must implement driver.DriverContext with logging capabilities and provides our RetryConnector.
type Driver struct {
	ctxDriver
	Logger logr.Logger
}

// OpenConnector implements the DriverContext interface.
func (d Driver) OpenConnector(name string) (driver.Connector, error) {
	c, err := d.ctxDriver.OpenConnector(name)
	if err != nil {
		return nil, err
	}

	return &RetryConnector{
		driver:    d,
		Connector: c,
	}, nil
}

// RegisterDrivers makes our database Driver(s) available under the name "icinga-*sql".
func RegisterDrivers(logger logr.Logger) {
	sql.Register(MySQL, &Driver{ctxDriver: &mysql.MySQLDriver{}, Logger: logger})
	sql.Register(PostgreSQL, &Driver{ctxDriver: &PgSQLDriver{}, Logger: logger})
	_ = mysql.SetLogger(mysqlLogger(func(v ...interface{}) { fmt.Println(v...) }))
	sqlx.BindDriver(PostgreSQL, sqlx.DOLLAR)
}

// ctxDriver helps ensure that we only support drivers that implement driver.Driver and driver.DriverContext.
type ctxDriver interface {
	driver.Driver
	driver.DriverContext
}

// mysqlLogger is an adapter that allows ordinary functions to be used as a logger for mysql.SetLogger.
type mysqlLogger func(v ...interface{})

// Print implements the mysql.Logger interface.
func (log mysqlLogger) Print(v ...interface{}) {
	log(v)
}

func shouldRetry(err error) bool {
	if errors.Is(err, driver.ErrBadConn) {
		return true
	}

	return retry.Retryable(err)
}
