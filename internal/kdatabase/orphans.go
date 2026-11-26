package kdatabase

import (
	"context"
	"fmt"

	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/com"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/pkg/errors"
)

// sharedForeignKey is the foreign key of the tables that rows of every resource
// kind share (label, annotation, favorite). Whether such a row is orphaned
// depends on the table of its own kind, which one parent cannot tell.
const sharedForeignKey = "resource_uuid"

// orphanStmt deletes the orphaned rows of one table.
type orphanStmt struct {
	table string
	query string
}

// orphanStmts returns the statements that delete the rows of parent's relations
// whose parent row no longer exists.
//
// It skips the relations that DeleteStreamedCascading would not delete either
// and the ones it cannot judge:
//   - relations without cascading deletes, whose rows have another owner,
//   - relations on a shared table (see sharedForeignKey),
//   - related entities with relations of their own, as deleting them flat
//     would orphan their children.
//
// Relations of one table through the same foreign key are swept once.
func orphanStmts(parent database.Entity) ([]orphanStmt, error) {
	relations, ok := parent.(HasRelations)
	if !ok {
		return nil, nil
	}

	type target struct{ table, foreignKey string }

	var stmts []orphanStmt
	seen := map[target]struct{}{}

	for _, rel := range relations.Relations() {
		if !rel.CascadeDelete() || rel.ForeignKey() == sharedForeignKey {
			continue
		}

		if _, ok := rel.NewEntity().(HasRelations); ok {
			continue
		}

		if rel.ForeignKey() == "" {
			return nil, errors.Errorf(
				"relation of %q to %q has no foreign key", database.TableName(parent), rel.TableName(),
			)
		}

		t := target{rel.TableName(), rel.ForeignKey()}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}

		stmts = append(stmts, orphanStmt{t.table, fmt.Sprintf(
			`DELETE FROM %q WHERE %q NOT IN (SELECT "uuid" FROM %q)`,
			t.table, t.foreignKey, database.TableName(parent),
		)})
	}

	return stmts, nil
}

// DeleteOrphans deletes the rows of parent's relations that point at a parent
// row which does not exist, in any cluster. Such rows are left behind when an
// upsert of a resource is written after its cascading delete, and nothing else
// ever removes them.
//
// Decide by "no parent row anywhere", never by cluster, so that rows of other
// clusters sharing the database stay untouched. Call it before the resource's
// own rows are written: a child row can precede its parent row while both are
// being written.
func (db *DB) DeleteOrphans(ctx context.Context, parent database.Entity) error {
	stmts, err := orphanStmts(parent)
	if err != nil {
		return err
	}

	for _, stmt := range stmts {
		if err := db.deleteOrphans(ctx, stmt); err != nil {
			return err
		}
	}

	return nil
}

// deleteOrphans executes stmt, one of orphanStmts, under the semaphore of its
// table, like the bulk writes to that table.
func (db *DB) deleteOrphans(ctx context.Context, stmt orphanStmt) error {
	var counter com.Counter

	sem := db.GetSemaphoreForTable(stmt.table)
	if err := sem.Acquire(ctx, 1); err != nil {
		return errors.Wrap(err, "cannot acquire semaphore")
	}
	defer sem.Release(1)

	defer db.Log(ctx, stmt.query, &counter).Stop()

	return retry.WithBackoff(
		ctx,
		func(ctx context.Context) error {
			rs, err := db.ExecContext(ctx, db.Rebind(stmt.query))
			if err != nil {
				return database.CantPerformQuery(err, stmt.query)
			}

			n, err := rs.RowsAffected()
			if err == nil && n > 0 {
				counter.Add(uint64(n))
			}

			return nil
		},
		retry.Retryable,
		backoff.DefaultBackoff,
		db.GetDefaultRetrySettings(),
	)
}
