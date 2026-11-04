package kdatabase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/com"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/periodic"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-go-library/utils"
)

const batchSize = 5000

// Cleaner deletes rows that are older than a given retention, see PeriodicCleanup.
type Cleaner interface {
	cleanup(ctx context.Context, db *DB, retention time.Duration) error
}

// DefaultCleaner defines information needed to compose cleanup statements.
type DefaultCleaner struct {
	Table string
	// PK lists the columns of the table's primary key.
	PK     []string
	Column string
}

// buildStmt assembles the cleanup statement for the specified database driver
// with the given limit.
func (c *DefaultCleaner) buildStmt(driverName string, limit uint64) string {
	switch driverName {
	case database.MySQL:
		return fmt.Sprintf(`DELETE FROM %q WHERE %q < :time LIMIT %d`, c.Table, c.Column, limit)
	case database.PostgreSQL:
		return fmt.Sprintf(
			`DELETE FROM %[2]q WHERE (%[1]s) IN (SELECT %[1]s FROM %[2]q WHERE %[3]q < :time LIMIT %[4]d)`,
			fmt.Sprintf(`"%s"`, strings.Join(c.PK, `", "`)),
			c.Table,
			c.Column,
			limit,
		)
	default:
		panic(fmt.Sprintf("invalid database type %s", driverName))
	}
}

// cleanup deletes all rows of the table whose time column is older than
// the retention, measured against the daemon's clock.
// Deletes a maximum of batchSize rows per round.
func (c *DefaultCleaner) cleanup(ctx context.Context, db *DB, retention time.Duration) error {
	var counter com.Counter

	query := db.Rebind(c.buildStmt(db.DriverName(), batchSize))
	olderThan := time.Now().Add(-retention)

	defer db.Log(ctx, query, &counter).Stop()

	for {
		var rowsDeleted int64

		if err := retry.WithBackoff(
			ctx,
			func(ctx context.Context) error {
				rs, err := db.NamedExecContext(ctx, query, cleanupWhere{
					Time: types.UnixMilli(olderThan),
				})
				if err != nil {
					return database.CantPerformQuery(err, query)
				}

				rowsDeleted, err = rs.RowsAffected()

				return err
			},
			retry.Retryable,
			backoff.DefaultBackoff,
			db.GetDefaultRetrySettings(),
		); err != nil {
			return err
		}

		counter.Add(uint64(rowsDeleted))

		if rowsDeleted < int64(batchSize) {
			break
		}
	}

	return nil
}

// CascadeCleaner defines an entity whose rows are cleaned up together with
// their relations based on the given time column.
type CascadeCleaner struct {
	Entity database.Entity

	// Column must hold Unix milliseconds set by the database clock.
	Column      string
	ClusterUuid types.UUID
}

// cleanup deletes all rows of the entity whose time column is older than the
// retention, measured against the database clock, in rounds of at most
// batchSize rows via DeleteStreamed with cascading, so that the relations of
// the entity are deleted as well.
func (c *CascadeCleaner) cleanup(ctx context.Context, db *DB, retention time.Duration) error {
	query := db.Rebind(db.BuildSelectStmt(c.Entity, &struct{ Uuid types.UUID }{}) + fmt.Sprintf(
		` WHERE %q < UNIX_TIMESTAMP() * 1000 - ? AND "cluster_uuid" = ? LIMIT %d`, c.Column, batchSize,
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
			db.GetDefaultRetrySettings(),
		); err != nil {
			return database.CantPerformQuery(err, query)
		}

		if err := db.DeleteStreamedCascading(ctx, c.Entity, utils.ChanFromSlice(uuids)); err != nil {
			return err
		}

		if len(uuids) < batchSize {
			return nil
		}
	}
}

type cleanupWhere struct {
	Time types.UnixMilli
}

// Cleanup deletes rows older than a given time in batches of limited size.
type Cleanup struct {
	db *DB
}

// NewCleanup returns a new Cleanup that deletes rows through db.
func NewCleanup(db *DB) *Cleanup {
	return &Cleanup{
		db: db,
	}
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

// PeriodicCleanup lets the given Cleaner delete the rows that are older than
// the retention once per interval in batches of batchSize. If no interval is
// set, the default is one hour. If no retention is set, the default is one day.
// It blocks until a cleanup fails or ctx is done, and returns the cleanup error
// or ctx.Err().
func (c *Cleanup) PeriodicCleanup(ctx context.Context, cleaner Cleaner, options ...CleanupOption) error {
	o := &cleanupOptions{
		interval:  time.Hour,
		retention: 24 * time.Hour,
	}
	for _, option := range options {
		option(o)
	}

	errs := make(chan error, 1)

	defer periodic.Start(ctx, o.interval, func(tick periodic.Tick) {
		if err := cleaner.cleanup(ctx, c.db, o.retention); err != nil {
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
