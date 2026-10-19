package kdatabase

import (
	"context"
	"database/sql/driver"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/creasty/defaults"
	"github.com/google/uuid"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/utils"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

const testTimeout = time.Second * 10

func TestUpsertStreamedCascadingWithoutRelations(t *testing.T) {
	t.Parallel()

	db, mock := newTestDB(t)
	upserted := &collector[database.Entity]{}

	l1, l2 := uuid.New(), uuid.New()
	labels := []database.Entity{&testLabel{Uuid: l1}, &testLabel{Uuid: l2}}

	mock.ExpectExec(upsert("test_label", 2)).WithArgs(l1, l2).WillReturnResult(sqlmock.NewResult(0, 2))

	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	t.Cleanup(cancel)

	if err := db.UpsertStreamedCascading(ctx, utils.ChanFromSlice(labels), upserted.onSuccess); err != nil {
		t.Fatal(err)
	}

	want := entityIds(labels...)
	got := entityIds(upserted.all()...)
	if !slices.Equal(got, want) {
		t.Errorf("callback: got %v, want %v", got, want)
	}
}

func TestUpsertStreamedCascadingWithRelations(t *testing.T) {
	t.Parallel()

	db, mock := newTestDB(t)
	upserted := &collector[database.Entity]{}

	p1, p2 := uuid.New(), uuid.New()
	l1, l2, l3, l4 := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	c1, t1, n1 := uuid.New(), uuid.New(), uuid.New()

	first := &testParent{
		Uuid:         p1,
		Labels:       []testLabel{{Uuid: l1}, {Uuid: l2}},
		PrimaryLabel: &testLabel{Uuid: l3},
		Children:     []*testChild{{Uuid: c1, Toys: []testToy{{Uuid: t1}}}},
		Notes:        []testNote{{Uuid: n1}},
	}
	second := &testParent{
		Uuid:   p2,
		Labels: []testLabel{{Uuid: l4}},
	}

	mock.ExpectExec(upsert("test_parent", 2)).WithArgs(p1, p2).WillReturnResult(sqlmock.NewResult(0, 2))
	// Labels and PrimaryLabel share the stream of their table, so they are
	// upserted together, but in no particular order, as each relation is
	// streamed concurrently. The nil PrimaryLabel of the second parent is
	// skipped.
	labels := oneOf{l1, l2, l3, l4}
	mock.ExpectExec(upsert("test_label", 4)).WithArgs(labels, labels, labels, labels).WillReturnResult(sqlmock.NewResult(0, 4))
	mock.ExpectExec(upsert("test_child", 1)).WithArgs(c1).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(upsert("test_toy", 1)).WithArgs(t1).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(upsert("test_note", 1)).WithArgs(n1).WillReturnResult(sqlmock.NewResult(0, 1))

	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	t.Cleanup(cancel)

	if err := db.UpsertStreamedCascading(ctx, utils.ChanFromSlice([]database.Entity{first, second}), upserted.onSuccess); err != nil {
		t.Fatal(err)
	}

	want := entityIds(first, second)
	got := entityIds(upserted.all()...)
	if !slices.Equal(got, want) {
		t.Errorf("callback: got %v, want %v", got, want)
	}
}

func TestUpsertStreamedCascadingWithNilCallback(t *testing.T) {
	t.Parallel()

	db, mock := newTestDB(t)

	l1 := uuid.New()

	mock.ExpectExec(upsert("test_label", 1)).WithArgs(l1).WillReturnResult(sqlmock.NewResult(0, 1))

	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	t.Cleanup(cancel)

	// Syncs without a callback pass nil, which icinga-go-library would call.
	if err := db.UpsertStreamedCascading(ctx, utils.ChanFromSlice([]database.Entity{&testLabel{Uuid: l1}}), nil); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteStreamedCascadingWithoutRelations(t *testing.T) {
	t.Parallel()

	db, mock := newTestDB(t)
	deleted := &collector[any]{}

	n1, n2 := uuid.New(), uuid.New()

	mock.ExpectExec(`DELETE FROM "test_note" WHERE uuid IN (?, ?)`).WithArgs(n1, n2).WillReturnResult(sqlmock.NewResult(0, 2))

	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	t.Cleanup(cancel)

	if err := db.DeleteStreamedCascading(ctx, &testNote{}, utils.ChanFromSlice([]any{n1, n2}), deleted.onSuccess); err != nil {
		t.Fatal(err)
	}

	want := strs(n1, n2)
	got := strs(deleted.all()...)
	if !slices.Equal(got, want) {
		t.Errorf("callback: got %v, want %v", got, want)
	}
}

func TestDeleteStreamedCascadingWithRelations(t *testing.T) {
	t.Parallel()

	db, mock := newTestDB(t)
	deleted := &collector[any]{}

	p1, p2 := uuid.New(), uuid.New()
	c1, c2, c3 := uuid.New(), uuid.New(), uuid.New()

	mock.ExpectExec(`DELETE FROM "test_parent" WHERE uuid IN (?, ?)`).WithArgs(p1, p2).WillReturnResult(sqlmock.NewResult(0, 2))
	// Labels and PrimaryLabel are both deleted by their foreign key.
	for range 2 {
		mock.ExpectExec(`DELETE FROM "test_label" WHERE parent_uuid IN (?, ?)`).WithArgs(p1, p2).WillReturnResult(sqlmock.NewResult(0, 2))
	}
	// Children have relations of their own, so their ids are looked up to
	// delete them and their toys by these ids. The ids are binary, as they are
	// scanned into types.UUID.
	mock.ExpectQuery(`SELECT uuid FROM "test_child" WHERE parent_uuid IN (?, ?)`).
		WithArgs(p1, p2).
		WillReturnRows(sqlmock.NewRows([]string{"uuid"}).AddRow(c1[:]).AddRow(c2[:]).AddRow(c3[:]))
	mock.ExpectExec(`DELETE FROM "test_child" WHERE uuid IN (?, ?, ?)`).WithArgs(c1[:], c2[:], c3[:]).WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectExec(`DELETE FROM "test_toy" WHERE child_uuid IN (?, ?, ?)`).WithArgs(c1[:], c2[:], c3[:]).WillReturnResult(sqlmock.NewResult(0, 3))
	// Notes are not deleted, as their relation is WithoutCascadeDelete.

	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	t.Cleanup(cancel)

	if err := db.DeleteStreamedCascading(ctx, &testParent{}, utils.ChanFromSlice([]any{p1, p2}), deleted.onSuccess); err != nil {
		t.Fatal(err)
	}

	want := strs(p1, p2)
	got := strs(deleted.all()...)
	if !slices.Equal(got, want) {
		t.Errorf("callback: got %v, want %v", got, want)
	}
}

func TestDeleteStreamedCascadingWithNilCallback(t *testing.T) {
	t.Parallel()

	db, mock := newTestDB(t)

	n1 := uuid.New()

	mock.ExpectExec(`DELETE FROM "test_note" WHERE uuid IN (?)`).WithArgs(n1).WillReturnResult(sqlmock.NewResult(0, 1))

	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	t.Cleanup(cancel)

	// Syncs without a callback pass nil, which icinga-go-library would call.
	if err := db.DeleteStreamedCascading(ctx, &testNote{}, utils.ChanFromSlice([]any{n1}), nil); err != nil {
		t.Fatal(err)
	}
}

func TestBuildDeleteStmt(t *testing.T) {
	t.Parallel()

	db := &DB{}

	if got, want := db.BuildDeleteStmt(&testNote{}), `DELETE FROM "test_note" WHERE uuid IN (?)`; got != want {
		t.Errorf("entity: got %q, want %q", got, want)
	}

	rel := HasMany([]testLabel(nil), WithForeignKey("parent_uuid"))
	if got, want := db.BuildDeleteStmt(rel), `DELETE FROM "test_label" WHERE parent_uuid IN (?)`; got != want {
		t.Errorf("relation: got %q, want %q", got, want)
	}
}

// testRow provides the uuid column and the database.Entity methods of the test entities.
type testRow struct {
	EntityStub
	Uuid uuid.UUID
}

func (r testRow) ID() database.ID { return r.Uuid }

// testParent has every kind of relation the cascades handle: plain ones, one
// to the same table, one to entities with relations of their own, and one
// without cascading deletes.
type testParent struct {
	testRow
	Labels       []testLabel  `db:"-"`
	PrimaryLabel *testLabel   `db:"-"`
	Children     []*testChild `db:"-"`
	Notes        []testNote   `db:"-"`
}

func (p *testParent) Relations() []Relation {
	fk := WithForeignKey("parent_uuid")

	return []Relation{
		HasMany(p.Labels, fk),
		HasOne(p.PrimaryLabel, fk),
		HasMany(p.Children, fk),
		HasMany(p.Notes, fk, WithoutCascadeDelete()),
	}
}

type testChild struct {
	testRow
	Toys []testToy `db:"-"`
}

func (c *testChild) Relations() []Relation {
	return []Relation{HasMany(c.Toys, WithForeignKey("child_uuid"))}
}

type testLabel struct {
	testRow
}

type testNote struct {
	testRow
}

type testToy struct {
	testRow
}

// newTestDB returns a MySQL DB backed by sqlmock. As the cascades execute
// their statements concurrently, sqlmock expects them in any order. The test
// fails if not all expected statements were executed by its end.
func newTestDB(t *testing.T) (*DB, sqlmock.Sqlmock) {
	t.Helper()

	cfg := &database.Config{}

	err := defaults.Set(cfg)
	if err != nil {
		t.Fatal(err)
	}

	db, err := NewFromConfig(cfg, logging.NewLogger(zap.NewNop().Sugar(), time.Second))
	if err != nil {
		t.Fatal(err)
	}

	conn, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	mock.MatchExpectationsInOrder(false)

	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}

		_ = conn.Close()
	})

	// NewFromConfig does not connect yet, so its connection pool can simply be
	// replaced. The mapper of icinga-go-library is kept, as it maps field names
	// to the same columns as the statements built from them.
	mocked := sqlx.NewDb(conn, database.MySQL)
	mocked.Mapper = db.Mapper
	_ = db.Close()
	db.DB.DB = mocked

	return db, mock
}

// upsert returns the statement that UpsertStreamed of icinga-go-library
// executes on MySQL to upsert rows of the test entities into table.
func upsert(table string, rows int) string {
	values := strings.TrimSuffix(strings.Repeat("(?),", rows), ",")

	return fmt.Sprintf(`INSERT INTO "%s" ("uuid") VALUES %s ON DUPLICATE KEY UPDATE "uuid" = VALUES("uuid")`, table, values)
}

// oneOf matches an argument that is one of the ids, for rows whose arguments
// are bound in no particular order.
type oneOf []uuid.UUID

func (ids oneOf) Match(v driver.Value) bool {
	return slices.ContainsFunc(ids, func(id uuid.UUID) bool {
		return v == id.String()
	})
}

// collector collects the rows passed to its onSuccess callback.
type collector[T any] struct {
	mu   sync.Mutex
	rows []T
}

func (c *collector[T]) onSuccess(_ context.Context, rows []T) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.rows = append(c.rows, rows...)

	return nil
}

func (c *collector[T]) all() []T {
	c.mu.Lock()
	defer c.mu.Unlock()

	return slices.Clone(c.rows)
}

// strs returns the sorted string forms of values.
func strs(values ...any) []string {
	s := make([]string, 0, len(values))
	for _, value := range values {
		s = append(s, fmt.Sprint(value))
	}
	slices.Sort(s)

	return s
}

// entityIds returns the sorted string forms of the ids of entities.
func entityIds(entities ...database.Entity) []string {
	ids := make([]any, 0, len(entities))
	for _, entity := range entities {
		ids = append(ids, entity.ID())
	}

	return strs(ids...)
}
