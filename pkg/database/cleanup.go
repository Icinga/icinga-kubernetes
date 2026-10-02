package database

import (
	"context"
	"fmt"
	"time"

	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/com"
	"github.com/icinga/icinga-go-library/periodic"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/icinga/icinga-go-library/types"
	"golang.org/x/sync/errgroup"
)

// CleanupStmt defines information needed to perform periodic cleanup.
type CleanupStmt struct {
	Table   string
	PK      string
	Column  string
	cleanup func(context.Context, time.Time) error
}

// Build assembles the cleanup statement for the specified database driver with the given limit.
func (stmt *CleanupStmt) Build(driverName string, limit uint64) string {
	switch driverName {
	case MySQL, "mysql":
		return fmt.Sprintf(`DELETE FROM %[1]s WHERE %[2]s < :time LIMIT %[3]d`, stmt.Table, stmt.Column, limit)
	case PostgreSQL, "postgres":
		return fmt.Sprintf(`WITH rows AS (SELECT %[1]s FROM %[2]s WHERE %[3]s < :time LIMIT %[4]d)
DELETE FROM %[2]s WHERE %[1]s IN (SELECT %[1]s FROM rows)`, stmt.PK, stmt.Table, stmt.Column, limit)
	default:
		panic(fmt.Sprintf("invalid database type %s", driverName))
	}
}

// CleanupOlderThan deletes all rows with the specified statement that are older
// than the given time. Deletes a maximum of as many rows per round as defined
// in count. Actually deleted rows will be passed to onSuccess. Returns the total
// number of rows deleted.
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
						db.log.Info("Cannot execute query. Retrying", "error", err)
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

type deletedPurgeCandidate struct {
	Uuid types.UUID
}

type deletedPurgeWhere struct {
	ClusterUuid types.UUID
	Time        types.UnixMilli
}

func (db *Database) buildDeletedPurgeSelectStmt(from any) string {
	return db.BuildSelectStmt(from, deletedPurgeCandidate{}) +
		` WHERE cluster_uuid=:cluster_uuid AND deleted IS NOT NULL AND deleted < :time`
}

func (db *Database) purgeDeletedBefore(
	ctx context.Context, from any, clusterUuid types.UUID, olderThan time.Time,
) error {
	g, ctx := errgroup.WithContext(ctx)

	entities, errs := db.YieldAll(
		ctx,
		func() (any, error) {
			return &deletedPurgeCandidate{}, nil
		},
		db.buildDeletedPurgeSelectStmt(from),
		deletedPurgeWhere{
			ClusterUuid: clusterUuid,
			Time:        types.UnixMilli(olderThan),
		},
	)
	com.ErrgroupReceive(g, errs)

	ids := make(chan any)
	g.Go(func() error {
		defer close(ids)

		for {
			select {
			case entity, more := <-entities:
				if !more {
					return nil
				}

				select {
				case ids <- entity.(*deletedPurgeCandidate).Uuid:
				case <-ctx.Done():
					return ctx.Err()
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	g.Go(func() error {
		return db.DeleteStreamed(
			ctx,
			from,
			ids,
			WithBlocking(),
			WithCascading(),
			withPurgeRelations(),
		)
	})

	return g.Wait()
}

// PeriodicPurgeDeleted physically removes soft-deleted resources after retention.
// Ordinary cascade relations remain authoritative, while purge-only relations
// participate only in this physical lifecycle phase.
func (db *Database) PeriodicPurgeDeleted(
	ctx context.Context, from any, clusterUuid types.UUID, retention time.Duration,
) error {
	if retention <= 0 {
		return fmt.Errorf("deleted retention must be greater than zero, got %s", retention)
	}

	return db.PeriodicCleanup(ctx, CleanupStmt{
		cleanup: func(ctx context.Context, now time.Time) error {
			return db.purgeDeletedBefore(ctx, from, clusterUuid, now.Add(-retention))
		},
	})
}

func (db *Database) PeriodicCleanup(ctx context.Context, stmt CleanupStmt) error {
	errs := make(chan error, 1)

	defer periodic.Start(ctx, time.Hour, func(tick periodic.Tick) {
		var err error
		if stmt.cleanup != nil {
			err = stmt.cleanup(ctx, tick.Time)
		} else {
			_, err = db.CleanupOlderThan(ctx, stmt, 5000, tick.Time.AddDate(0, 0, -1))
		}

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
