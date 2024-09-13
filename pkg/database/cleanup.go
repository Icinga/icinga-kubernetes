package database

import (
	"context"
	"fmt"
	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/com"
	"github.com/icinga/icinga-kubernetes/pkg/periodic"
	"golang.org/x/sync/errgroup"
	"time"
)

// CleanupStmt defines information needed to compose cleanup statements.
type CleanupStmt struct {
	Table  string
	PK     string
	Column string
}

// Build assembles the cleanup statement for the specified database driver with the given limit.
func (stmt *CleanupStmt) Build(driverName string, limit uint64) string {
	switch driverName {
	case MySQL, "mysql":
		return fmt.Sprintf(`DELETE FROM %[1]s WHERE %[2]s < :time
ORDER BY %[2]s LIMIT %[3]d`, stmt.Table, stmt.Column, limit)
	case PostgreSQL, "postgres":
		return fmt.Sprintf(`WITH rows AS (
SELECT %[1]s FROM %[2]s WHERE %[3]s < :time ORDER BY %[3]s LIMIT %[4]d
)
DELETE FROM %[2]s WHERE %[1]s IN (SELECT %[1]s FROM rows)`, stmt.PK, stmt.Table, stmt.Column, limit)
	default:
		panic(fmt.Sprintf("invalid database type %s", driverName))
	}
}

// CleanupOlderThan deletes all rows with the specified statement that are older than the given time.
// Deletes a maximum of as many rows per round as defined in count. Actually deleted rows will be passed to onSuccess.
// Returns the total number of rows deleted.
func (db *Database) CleanupOlderThan(
	ctx context.Context, stmt CleanupStmt,
	count uint64, olderThan time.Time, onSuccess ...OnSuccess[struct{}],
) (uint64, error) {
	var counter com.Counter

	q := db.Rebind(stmt.Build(db.DriverName(), count))

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
						db.log.Info("Can't execute query. Retrying", "error", err)
					}
				},
				OnSuccess: func(elapsed time.Duration, attempt uint64, lastErr error) {
					if attempt > 1 {
						db.log.Info("Query retried successfully after error",
							"after", elapsed, "attempt", attempt, "recovered_error", lastErr)
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

		if rowsDeleted < int64(count) {
			break
		}
	}

	return counter.Total(), nil
}

type cleanupWhere struct {
	Time types.UnixMilli
}

func (db *Database) PeriodicCleanup(ctx context.Context, stmt CleanupStmt) error {
	g, ctxCleanup := errgroup.WithContext(ctx)

	errs := make(chan error, 1)
	defer close(errs)

	periodic.Start(ctx, time.Hour, func(tick periodic.Tick) {
		olderThan := tick.Time.AddDate(0, 0, -1)

		_, err := db.CleanupOlderThan(
			ctx, stmt, 5000, olderThan,
		)

		if err != nil {
			select {
			case errs <- err:
			case <-ctx.Done():
			}

			return
		}
	}, periodic.Immediate()).Stop()

	com.ErrgroupReceive(ctxCleanup, g, errs)

	return g.Wait()
}
