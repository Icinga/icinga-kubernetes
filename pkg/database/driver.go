package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"go.uber.org/zap"
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
			OnRetryableError: func(_ time.Duration, _ uint64, err, lastErr error) {
				if lastErr == nil || err.Error() != lastErr.Error() {
					c.driver.Logger.Warnw("Cannot connect to database. Retrying", zap.Error(err))
				}
			},
			OnSuccess: func(elapsed time.Duration, attempt uint64, _ error) {
				if attempt > 1 {
					c.driver.Logger.Infow(
						"Reconnected to database",
						zap.Duration("after", elapsed),
						zap.Uint64("attempts", attempt),
					)
				}
			},
		},
	), "cannot connect to database")
	return conn, err
}

// Driver implements part of the driver.Connector interface.
func (c RetryConnector) Driver() driver.Driver {
	return c.driver
}

// Driver wraps a driver.Driver that also must implement driver.DriverContext with logging capabilities and provides our RetryConnector.
type Driver struct {
	ctxDriver
	Logger *logging.Logger
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
func RegisterDrivers(logger *logging.Logger) {
	sql.Register(MySQL, &Driver{ctxDriver: &mysql.MySQLDriver{}, Logger: logger})
	sql.Register(PostgreSQL, &Driver{ctxDriver: &PgSQLDriver{}, Logger: logger})
	_ = mysql.SetLogger(database.MysqlFuncLogger(logger.Debug))
	sqlx.BindDriver(PostgreSQL, sqlx.DOLLAR)
}

// ctxDriver helps ensure that we only support drivers that implement driver.Driver and driver.DriverContext.
type ctxDriver interface {
	driver.Driver
	driver.DriverContext
}

func shouldRetry(err error) bool {
	if errors.Is(err, driver.ErrBadConn) {
		return true
	}

	return retry.Retryable(err)
}
