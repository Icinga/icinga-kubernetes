package database

import (
	"context"
	"fmt"
	"time"

	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/com"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/periodic"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-go-library/utils"
	"go.uber.org/zap"
)

const batchSize = 5000

// Cleaner deletes rows that are older than a given retention, see PeriodicCleanup.
type Cleaner interface {
	cleanup(ctx context.Context, db *Database, retention time.Duration) error
}

// DefaultCleaner defines information needed to compose cleanup statements.
type DefaultCleaner struct {
	Table  string
	PK     string
	Column string
}

// buildStmt assembles the cleanup statement for the specified database driver
// with the given limit.
func (c *DefaultCleaner) buildStmt(driverName string, limit uint64) string {
	switch driverName {
	case MySQL, "mysql":
		return fmt.Sprintf(`DELETE FROM %[1]s WHERE %[2]s < :time LIMIT %[3]d`, c.Table, c.Column, limit)
	case PostgreSQL, "postgres":
		return fmt.Sprintf(`WITH rows AS (SELECT %[1]s FROM %[2]s WHERE %[3]s < :time LIMIT %[4]d)
DELETE FROM %[2]s WHERE %[1]s IN (SELECT %[1]s FROM rows)`, c.PK, c.Table, c.Column, limit)
	default:
		panic(fmt.Sprintf("invalid database type %s", driverName))
	}
}

func (c *DefaultCleaner) cleanup(ctx context.Context, db *Database, retention time.Duration) error {
	_, err := db.CleanupOlderThan(ctx, c, time.Now().Add(-retention))

	return err
}

// CascadeCleaner defines an entity whose rows are cleaned up together with
// their relations based on the given time column.
type CascadeCleaner struct {
	Entity HasRelations

	// Column must hold Unix milliseconds set by the database clock.
	Column      string
	ClusterUuid types.UUID
}

// cleanup deletes all rows of the entity whose time column is older than the
// retention, measured against the database clock, in rounds of at most
// batchSize rows via DeleteStreamed with cascading, so that the relations of
// the entity are deleted as well.
func (c *CascadeCleaner) cleanup(ctx context.Context, db *Database, retention time.Duration) error {
	query := db.Rebind(db.BuildSelectStmt(c.Entity, &struct{ Uuid types.UUID }{}) + fmt.Sprintf(
		" WHERE %s < UNIX_TIMESTAMP() * 1000 - ? AND %s = ? LIMIT %d",
		db.QuoteIdentifier(c.Column),
		db.QuoteIdentifier("cluster_uuid"),
		batchSize,
	))

	for {
		// Scanned into []any so that the UUIDs can feed DeleteStreamed's <-chan any.
		var uuids []any

		if err := retry.WithBackoff(
			ctx,
			func(ctx context.Context) error {
				uuids = nil

				return db.SelectContext(ctx, &uuids, query, retention.Milliseconds(), c.ClusterUuid)
			},
			retry.Retryable,
			backoff.DefaultBackoff,
			retry.Settings{},
		); err != nil {
			return CantPerformQuery(err, query)
		}

		if err := db.DeleteStreamed(ctx, c.Entity, utils.ChanFromSlice(uuids), WithCascading()); err != nil {
			return err
		}

		if len(uuids) < batchSize {
			return nil
		}
	}
}

// CleanupOlderThan deletes all rows with the specified statement that are older
// than the given time. Deletes a maximum of batchSize rows per round. Actually
// deleted rows will be passed to onSuccess. Returns the total number of rows
// deleted.
func (db *Database) CleanupOlderThan(
	ctx context.Context, stmt *DefaultCleaner, olderThan time.Time, onSuccess ...OnSuccess[struct{}],
) (uint64, error) {
	var counter com.Counter

	q := db.Rebind(stmt.buildStmt(db.DriverName(), batchSize))

	defer db.periodicLog(ctx, q, &counter).Stop()

	for {
		var rowsDeleted int64

		err := retry.WithBackoff(
			ctx,
			func(ctx context.Context) error {
				rs, err := db.NamedExecContext(ctx, q, cleanupWhere{
					Time: types.UnixMilli(olderThan),
				})
				if err != nil {
					return CantPerformQuery(err, q)
				}

				rowsDeleted, err = rs.RowsAffected()

				return err
			},
			retry.Retryable,
			backoff.NewExponentialWithJitter(1*time.Millisecond, 1*time.Second),
			retry.Settings{
				Timeout: retry.DefaultTimeout,
				OnRetryableError: func(_ time.Duration, _ uint64, err, lastErr error) {
					if lastErr == nil || err.Error() != lastErr.Error() {
						db.log.Warnw("Cannot execute query. Retrying", logging.Error(err))
					}
				},
				OnSuccess: func(elapsed time.Duration, attempt uint64, lastErr error) {
					if attempt > 1 {
						db.log.Infow(
							"Query retried successfully after error",
							zap.Duration("after", elapsed),
							zap.Uint64("attempts", attempt),
							logging.Error(lastErr),
						)
					}
				},
			},
		)
		if err != nil {
			return 0, err
		}

		counter.Add(uint64(rowsDeleted))

		for _, onSuccess := range onSuccess {
			if err := onSuccess(ctx, make([]struct{}, rowsDeleted)); err != nil {
				return 0, err
			}
		}

		if rowsDeleted < int64(batchSize) {
			break
		}
	}

	return counter.Total(), nil
}

type cleanupWhere struct {
	Time types.UnixMilli
}

// CleanupOption configures PeriodicCleanup.
type CleanupOption func(*cleanupOptions)

type cleanupOptions struct {
	interval  time.Duration
	retention time.Duration
}

// WithInterval sets how often PeriodicCleanup runs. Defaults to one hour.
func WithInterval(interval time.Duration) CleanupOption {
	return func(o *cleanupOptions) {
		o.interval = interval
	}
}

// WithRetention sets how long rows are kept before PeriodicCleanup deletes
// them. Defaults to 24 hours.
func WithRetention(retention time.Duration) CleanupOption {
	return func(o *cleanupOptions) {
		o.retention = retention
	}
}

// PeriodicCleanup deletes rows with the specified statement that are older
// than the retention once per interval.
func (db *Database) PeriodicCleanup(ctx context.Context, stmt Cleaner, options ...CleanupOption) error {
	o := &cleanupOptions{
		interval:  time.Hour,
		retention: 24 * time.Hour,
	}
	for _, option := range options {
		option(o)
	}

	errs := make(chan error, 1)

	defer periodic.Start(ctx, o.interval, func(periodic.Tick) {
		if err := stmt.cleanup(ctx, db, o.retention); err != nil {
			select {
			case errs <- err:
			case <-ctx.Done():
			}

			return
		}
	}, periodic.Immediate()).Stop()

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
