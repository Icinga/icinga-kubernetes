package database

import (
	"context"
	"fmt"
	"sync"

	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/com"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/icinga/icinga-go-library/types"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// DB is a wrapper around database.DB with additional relation handling.
type DB struct {
	*database.DB

	log *logging.Logger
}

// NewFromConfig returns a new DB for the database configured in cfg. It uses
// NewDbFromConfig of icinga-go-library, which also sets the ANSI_QUOTES SQL
// mode for MySQL that the built statements rely on. No connection is opened
// yet, so use Connect to check that the database is reachable. All database
// logs go to log.
func NewFromConfig(cfg *database.Config, log *logging.Logger) (*DB, error) {
	db, err := database.NewDbFromConfig(cfg, log, database.RetryConnectorCallbacks{})
	if err != nil {
		return nil, err
	}

	return &DB{
		DB:  db,
		log: log,
	}, nil
}

// BuildDeleteStmt returns a DELETE statement for the given struct.
func (db *DB) BuildDeleteStmt(from any) string {
	var column string
	if relation, ok := from.(Relation); ok {
		column = relation.ForeignKey()
	} else {
		column = "uuid"
	}

	return fmt.Sprintf(
		`DELETE FROM "%s" WHERE %s IN (?)`,
		database.TableName(from),
		column,
	)
}

// BulkExec bulk executes queries with a single slice placeholder in the form
// of `IN (?)`. Takes in up to the number of arguments specified in count from
// the arg stream, derives and expands a query and executes it with this set of
// arguments until the arg stream has been processed. The derived queries are
// executed in a separate goroutine with a weighting of 1 and can be executed
// concurrently to the extent allowed by the semaphore passed in sem.
// Arguments for which the query ran successfully will be passed to onSuccess.
func (db *DB) BulkExec(
	ctx context.Context, query string, count int, sem *semaphore.Weighted, arg <-chan any, features ...Feature,
) error {
	var counter com.Counter
	defer db.Log(ctx, query, &counter).Stop()

	g, ctx := errgroup.WithContext(ctx)

	f := NewFeatures(features...)
	var n int64
	if f.blocking {
		n = int64(db.Options.MaxConnectionsPerTable)
	} else {
		n = 1
	}

	bulk := com.Bulk(ctx, arg, count, com.NeverSplit[any])
	g.Go(func() error {
		g, ctx := errgroup.WithContext(ctx)

		for b := range bulk {
			if err := sem.Acquire(ctx, n); err != nil {
				return errors.Wrap(err, "cannot acquire semaphore")
			}

			g.Go(func(b []any) func() error {
				return func() error {
					defer sem.Release(n)

					return retry.WithBackoff(
						ctx,
						func(context.Context) error {
							stmt, args, err := sqlx.In(query, b)
							if err != nil {
								return errors.Wrapf(err, "cannot build placeholders for %q", query)
							}

							stmt = db.Rebind(stmt)
							_, err = db.ExecContext(ctx, stmt, args...)
							if err != nil {
								return database.CantPerformQuery(err, query)
							}

							counter.Add(uint64(len(b)))

							if f.onSuccess != nil {
								if err := f.onSuccess(ctx, b); err != nil {
									return err
								}
							}

							return nil
						},
						retry.Retryable,
						backoff.DefaultBackoff,
						db.GetDefaultRetrySettings(),
					)
				}
			}(b))
		}

		return g.Wait()
	})

	return g.Wait()
}

func (db *DB) Connect() bool {
	db.log.Info("Connecting to database")
	if err := db.Ping(); err != nil {
		db.log.Errorw("Cannot connect to database", logging.Error(err))

		return false
	}

	return true
}

// NamedBulkExec bulk executes queries with named placeholders in a VALUES
// clause most likely in the format INSERT ... VALUES. Takes in up to the number
// of entities specified in count from the arg stream, derives and executes a new
// query with the VALUES clause expanded to this set of arguments, until the arg
// stream has been processed. The queries are executed in a separate goroutine
// with a weighting of 1 and can be executed concurrently to the extent allowed
// by the semaphore passed in sem. Entities for which the query ran successfully
// will be passed to onSuccess.
func (db *DB) NamedBulkExec(
	ctx context.Context, query string, count int, sem *semaphore.Weighted, arg <-chan any,
	splitPolicyFactory com.BulkChunkSplitPolicyFactory[any], onSuccess ...database.OnSuccess[any],
) error {
	var counter com.Counter
	defer db.Log(ctx, query, &counter).Stop()

	g, ctx := errgroup.WithContext(ctx)
	bulk := com.Bulk(ctx, arg, count, splitPolicyFactory)

	g.Go(func() error {
		for {
			select {
			case b, ok := <-bulk:
				if !ok {
					return nil
				}

				if err := sem.Acquire(ctx, 1); err != nil {
					return errors.Wrap(err, "cannot acquire semaphore")
				}

				g.Go(func(b []any) func() error {
					return func() error {
						defer sem.Release(1)

						return retry.WithBackoff(
							ctx,
							func(ctx context.Context) error {
								_, err := db.NamedExecContext(ctx, query, b)
								if err != nil {
									return database.CantPerformQuery(err, query)
								}

								counter.Add(uint64(len(b)))

								for _, onSuccess := range onSuccess {
									if err := onSuccess(ctx, b); err != nil {
										return err
									}
								}

								return nil
							},
							retry.Retryable,
							backoff.DefaultBackoff,
							db.GetDefaultRetrySettings(),
						)
					}
				}(b))
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	return g.Wait()
}

// streamChildIds streams the ids of the rows of child that reference one of the
// parent ids through the foreign key of relation into childIds.
func (db *DB) streamChildIds(
	ctx context.Context, child any, relation Relation, parentIds <-chan any, childIds chan<- any,
) error {
	query := fmt.Sprintf(
		`SELECT uuid FROM "%s" WHERE %s IN (?)`,
		database.TableName(child),
		relation.ForeignKey(),
	)

	for batch := range com.Bulk(ctx, parentIds, db.Options.MaxPlaceholdersPerStatement, com.NeverSplit[any]) {
		var ids []types.UUID

		stmt, args, err := sqlx.In(query, batch)
		if err != nil {
			return errors.Wrapf(err, "cannot build placeholders for %q", query)
		}

		if err := retry.WithBackoff(
			ctx,
			func(ctx context.Context) error {
				ids = nil

				return db.SelectContext(ctx, &ids, db.Rebind(stmt), args...)
			},
			retry.Retryable,
			backoff.DefaultBackoff,
			db.GetDefaultRetrySettings(),
		); err != nil {
			return database.CantPerformQuery(err, query)
		}

		for _, id := range ids {
			select {
			case childIds <- id:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return ctx.Err()
}

// DeleteStreamed bulk deletes the specified ids via BulkExec. The delete
// statement is created using BuildDeleteStmt(from). Bulk size is controlled
// via Options.MaxPlaceholdersPerStatement and concurrency is controlled via
// Options.MaxConnectionsPerTable. With cascading, related entities that have
// relations themselves are deleted recursively by their own ids. IDs for which
// the query ran successfully will be passed to onSuccess.
func (db *DB) DeleteStreamed(
	ctx context.Context, from any, ids <-chan any, features ...Feature,
) error {
	f := NewFeatures(features...)

	if relations, ok := from.(HasRelations); ok && f.cascading {
		var g *errgroup.Group
		g, ctx = errgroup.WithContext(ctx)
		streams := make([]chan any, 0, len(relations.Relations()))
		for _, relation := range relations.Relations() {
			if !relation.CascadeDelete() {
				continue
			}

			ch := make(chan any)
			if child, ok := relation.NewEntity().(HasRelations); ok {
				childIds := make(chan any)
				g.Go(func() error {
					defer close(childIds)

					return db.streamChildIds(ctx, child, relation, ch, childIds)
				})
				g.Go(func() error {
					// onSuccess expects ids of the entity the deletion started
					// from, not those of its children.
					return db.DeleteStreamed(ctx, child, childIds, f.withoutOnSuccess())
				})
			} else {
				g.Go(func() error {
					return db.DeleteStreamed(ctx, relation, ch, features...)
				})
			}
			streams = append(streams, ch)
		}

		source := ids
		ids := make(chan any)
		dup := make(chan any)

		g.Go(func() error {
			defer close(ids)
			defer close(dup)

			for {
				select {
				case entity, more := <-source:
					if !more {
						return nil
					}

					select {
					case ids <- entity:
					case <-ctx.Done():
						return ctx.Err()
					}

					select {
					case dup <- entity:
					case <-ctx.Done():
						return ctx.Err()
					}
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})

		g.Go(func() error {
			defer func() {
				for _, ch := range streams {
					close(ch)
				}
			}()

			for {
				select {
				case entity, more := <-dup:
					if !more {
						return nil
					}

					for _, ch := range streams {
						select {
						case ch <- entity:
						case <-ctx.Done():
							return ctx.Err()
						}
					}
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})

		g.Go(func() error {
			return db.BulkExec(
				ctx,
				db.BuildDeleteStmt(from),
				db.Options.MaxPlaceholdersPerStatement,
				db.GetSemaphoreForTable(database.TableName(from)),
				ids,
				features...,
			)
		})

		return g.Wait()
	}

	return db.BulkExec(
		ctx,
		db.BuildDeleteStmt(from),
		db.Options.MaxPlaceholdersPerStatement,
		db.GetSemaphoreForTable(database.TableName(from)),
		ids,
		features...,
	)
}

// UpsertStreamed bulk upserts the specified entities via NamedBulkExec.
// The upsert statement is created using BuildUpsertStmt with the first
// entity from the entities stream. Bulk size is controlled via
// Options.MaxPlaceholdersPerStatement and concurrency is controlled
// via Options.MaxConnectionsPerTable.
func (db *DB) UpsertStreamed(
	ctx context.Context, entities <-chan any, features ...Feature,
) error {
	first, forward, err := com.CopyFirst(ctx, entities)
	if first == nil {
		return errors.Wrap(err, "cannot copy first entity")
	}

	sem := db.GetSemaphoreForTable(database.TableName(first))
	stmt, placeholders := db.BuildUpsertStmt(first)
	with := NewFeatures(features...)

	var onSuccess []database.OnSuccess[any]
	if with.onSuccess != nil {
		onSuccess = append(onSuccess, with.onSuccess)
	}

	if relations, ok := first.(HasRelations); ok && with.cascading {
		var g *errgroup.Group
		g, ctx = errgroup.WithContext(ctx)
		streams := make(map[string]chan any, len(relations.Relations()))
		for _, relation := range relations.Relations() {
			// Relations to the same table share its stream,
			// as the entities are looked up by table name.
			if _, exists := streams[database.TableName(relation)]; exists {
				continue
			}

			ch := make(chan any)
			g.Go(func() error {
				return db.UpsertStreamed(ctx, ch, WithCascading())
			})
			streams[database.TableName(relation)] = ch
		}

		source := forward
		forward := make(chan any)
		dup := make(chan any)

		g.Go(func() error {
			defer close(forward)
			defer close(dup)

			for {
				select {
				case entity, more := <-source:
					if !more {
						return nil
					}

					select {
					case forward <- entity:
					case <-ctx.Done():
						return ctx.Err()
					}

					select {
					case dup <- entity:
					case <-ctx.Done():
						return ctx.Err()
					}
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})

		g.Go(func() error {
			var senders sync.WaitGroup

			defer func() {
				senders.Wait()
				for _, ch := range streams {
					close(ch)
				}
			}()

			for {
				select {
				case entity, more := <-dup:
					if !more {
						return nil
					}

					for _, relation := range entity.(HasRelations).Relations() {
						senders.Add(1)
						g.Go(func() error {
							defer senders.Done()

							return relation.StreamInto(ctx, streams[database.TableName(relation)])
						})
					}
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})

		g.Go(func() error {
			return db.NamedBulkExec(
				ctx, stmt, db.BatchSizeByPlaceholders(placeholders), sem, forward, com.NeverSplit[any], onSuccess...)
		})

		return g.Wait()
	}

	return db.NamedBulkExec(
		ctx, stmt, db.BatchSizeByPlaceholders(placeholders), sem, forward, com.NeverSplit[any], onSuccess...)
}
