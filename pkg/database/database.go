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

func (db *DB) Connect() bool {
	db.log.Info("Connecting to database")
	if err := db.Ping(); err != nil {
		db.log.Errorw("Cannot connect to database", logging.Error(err))

		return false
	}

	return true
}

// streamChildIds streams the ids of the related rows that reference one of the
// parent ids through the foreign key of relation into childIds.
func (db *DB) streamChildIds(ctx context.Context, relation Relation, parentIds <-chan any, childIds chan<- any) error {
	query := fmt.Sprintf(
		`SELECT uuid FROM "%s" WHERE %s IN (?)`,
		database.TableName(relation),
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

// deleteStreamed deletes the rows of from, an entity or a relation, whose uuid
// or, for a relation, whose foreign key is one of ids.
//
// It replaces DeleteStreamed of icinga-go-library, which always deletes by an
// id column and only takes entities. Defining BuildDeleteStmt locally does not
// change that, as the library's DeleteStreamed always calls its own.
func (db *DB) deleteStreamed(ctx context.Context, from any, ids <-chan any, onSuccess ...database.OnSuccess[any]) error {
	return db.BulkExec(
		ctx,
		db.BuildDeleteStmt(from),
		db.Options.MaxPlaceholdersPerStatement,
		db.GetSemaphoreForTable(database.TableName(from)),
		ids,
		onSuccess...,
	)
}

// DeleteStreamedCascading deletes the rows of from whose uuid is one of ids
// and, recursively, the related rows of each relation that cascades on delete.
// All tables are deleted concurrently and in no particular order.
//
// onSuccess receives the ids once the rows of from are deleted.
func (db *DB) DeleteStreamedCascading(
	ctx context.Context, from database.Entity, ids <-chan any, onSuccess ...database.OnSuccess[any],
) error {
	forwardOnSuccess := withoutNil(onSuccess)

	relations, ok := from.(HasRelations)
	if !ok {
		return db.deleteStreamed(ctx, from, ids, forwardOnSuccess...)
	}

	g, ctx := errgroup.WithContext(ctx)
	streams := make([]chan any, 0, len(relations.Relations()))
	for _, rel := range relations.Relations() {
		if !rel.CascadeDelete() {
			continue
		}

		ch := make(chan any)
		child := rel.NewEntity()
		if _, ok := child.(HasRelations); ok {
			childIds := make(chan any)
			g.Go(func() error {
				defer close(childIds)

				return db.streamChildIds(ctx, rel, ch, childIds)
			})
			g.Go(func() error {
				return db.DeleteStreamedCascading(ctx, child, childIds)
			})
		} else {
			g.Go(func() error {
				return db.deleteStreamed(ctx, rel, ch)
			})
		}
		streams = append(streams, ch)
	}

	parents := make(chan any)
	dup := make(chan any)

	g.Go(func() error {
		defer close(parents)
		defer close(dup)

		for {
			select {
			case entity, more := <-ids:
				if !more {
					return nil
				}

				select {
				case parents <- entity:
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
		return db.deleteStreamed(ctx, from, parents, forwardOnSuccess...)
	})

	return g.Wait()
}

// UpsertStreamedCascading upserts entities and, recursively, the entities of
// their relations. All tables are upserted concurrently and in no particular
// order, each with a statement built from its first entity, so a stream must
// carry a single type.
//
// onSuccess receives only the entities from entities.
func (db *DB) UpsertStreamedCascading(
	ctx context.Context, entities <-chan database.Entity, onSuccess ...database.OnSuccess[any],
) error {
	first, forward, err := com.CopyFirst(ctx, entities)
	if err != nil {
		return errors.Wrap(err, "cannot copy first entity")
	}

	var forwardOnSuccess []database.OnSuccess[database.Entity]
	for _, fn := range withoutNil(onSuccess) {
		forwardOnSuccess = append(forwardOnSuccess, func(ctx context.Context, entities []database.Entity) error {
			rows := make([]any, 0, len(entities))
			for _, entity := range entities {
				rows = append(rows, entity)
			}

			return fn(ctx, rows)
		})
	}

	relations, ok := first.(HasRelations)
	if !ok {
		return db.UpsertStreamed(ctx, forward, forwardOnSuccess...)
	}

	g, ctx := errgroup.WithContext(ctx)
	streams := make(map[string]chan database.Entity, len(relations.Relations()))
	for _, rel := range relations.Relations() {
		// Relations to the same table share its stream,
		// as the entities are looked up by table name.
		if _, exists := streams[database.TableName(rel)]; exists {
			continue
		}

		ch := make(chan database.Entity)
		g.Go(func() error {
			return db.UpsertStreamedCascading(ctx, ch)
		})
		streams[database.TableName(rel)] = ch
	}

	parents := make(chan database.Entity)
	dup := make(chan database.Entity)

	g.Go(func() error {
		defer close(parents)
		defer close(dup)

		for {
			select {
			case entity, more := <-forward:
				if !more {
					return nil
				}

				select {
				case parents <- entity:
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

				for _, rel := range entity.(HasRelations).Relations() {
					senders.Add(1)
					g.Go(func() error {
						defer senders.Done()

						return rel.StreamInto(ctx, streams[database.TableName(rel)])
					})
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	g.Go(func() error {
		return db.UpsertStreamed(ctx, parents, forwardOnSuccess...)
	})

	return g.Wait()
}

// withoutNil returns fns without nil callbacks, which the library would call nevertheless.
func withoutNil[T any](fns []database.OnSuccess[T]) []database.OnSuccess[T] {
	var output []database.OnSuccess[T]
	for _, fn := range fns {
		if fn != nil {
			output = append(output, fn)
		}
	}

	return output
}
