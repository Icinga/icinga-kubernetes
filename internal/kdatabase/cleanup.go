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
)

const batchSize = 5000

// CleanupStmt defines information needed to compose cleanup statements.
type CleanupStmt struct {
	Table string
	// PK lists the columns of the table's primary key.
	PK     []string
	Column string
}

// Build assembles the cleanup statement for the specified database driver with the given limit.
func (stmt *CleanupStmt) Build(driverName string, limit uint64) string {
	switch driverName {
	case database.MySQL:
		return fmt.Sprintf(`DELETE FROM %q WHERE %q < :time LIMIT %d`, stmt.Table, stmt.Column, limit)
	case database.PostgreSQL:
		return fmt.Sprintf(
			`DELETE FROM %[2]q WHERE (%[1]s) IN (SELECT %[1]s FROM %[2]q WHERE %[3]q < :time LIMIT %[4]d)`,
			fmt.Sprintf(`"%s"`, strings.Join(stmt.PK, `", "`)),
			stmt.Table,
			stmt.Column,
			limit,
		)
	default:
		panic(fmt.Sprintf("invalid database type %s", driverName))
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

// CleanupOlderThan deletes all rows with the specified statement that are older
// than the given time. Deletes a maximum of batchSize rows per round. Actually
// deleted rows will be passed to onSuccess. Returns the total number of rows
// deleted.
func (c *Cleanup) CleanupOlderThan(
	ctx context.Context, stmt CleanupStmt, olderThan time.Time, onSuccess ...database.OnSuccess[struct{}],
) (uint64, error) {
	var counter com.Counter

	q := c.db.Rebind(stmt.Build(c.db.DriverName(), batchSize))

	defer c.db.Log(ctx, q, &counter).Stop()

	for {
		var rowsDeleted int64

		err := retry.WithBackoff(
			ctx,
			func(ctx context.Context) error {
				rs, err := c.db.NamedExecContext(ctx, q, cleanupWhere{
					Time: types.UnixMilli(olderThan),
				})
				if err != nil {
					return database.CantPerformQuery(err, q)
				}

				rowsDeleted, err = rs.RowsAffected()

				return err
			},
			retry.Retryable,
			backoff.DefaultBackoff,
			c.db.GetDefaultRetrySettings(),
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

// PeriodicCleanup deletes the rows of stmt that are older than one day, once
// right away and then every hour, in batches of 5000. It blocks until a cleanup
// fails or ctx is done, and returns the cleanup error or ctx.Err().
func (c *Cleanup) PeriodicCleanup(ctx context.Context, stmt CleanupStmt) error {
	errs := make(chan error, 1)

	defer periodic.Start(ctx, time.Hour, func(tick periodic.Tick) {
		olderThan := tick.Time.AddDate(0, 0, -1)

		_, err := c.CleanupOlderThan(ctx, stmt, olderThan)
		if err != nil {
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
