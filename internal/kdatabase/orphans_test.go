package kdatabase

import (
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// testSharedParent writes a table that rows of other resource kinds share.
type testSharedParent struct {
	testRow
	Labels []testLabel `db:"-"`
}

func (p *testSharedParent) Relations() []Relation {
	return []Relation{HasMany(p.Labels, WithForeignKey("resource_uuid"))}
}

// testNoKeyParent has a relation that does not name its foreign key.
type testNoKeyParent struct {
	testRow
	Labels []testLabel `db:"-"`
}

func (p *testNoKeyParent) Relations() []Relation {
	return []Relation{HasMany(p.Labels)}
}

// testTwoKeysParent relates the same table through two foreign keys.
type testTwoKeysParent struct {
	testRow
	Labels []testLabel `db:"-"`
	Others []testLabel `db:"-"`
}

func (p *testTwoKeysParent) Relations() []Relation {
	return []Relation{
		HasMany(p.Labels, WithForeignKey("parent_uuid")),
		HasMany(p.Others, WithForeignKey("other_uuid")),
	}
}

func TestDeleteOrphans(t *testing.T) {
	t.Parallel()

	db, mock := newTestDB(t)

	// testParent relates test_label twice (HasMany and HasOne, same foreign key),
	// test_child, which has relations of its own, and test_note, which does not
	// cascade. Only test_label is swept, and once.
	mock.ExpectExec(
		`DELETE FROM "test_label" WHERE "parent_uuid" NOT IN (SELECT "uuid" FROM "test_parent")`,
	).WillReturnResult(sqlmock.NewResult(0, 3))

	if err := db.DeleteOrphans(t.Context(), &testParent{}); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteOrphansSkipsSharedTables(t *testing.T) {
	t.Parallel()

	db, _ := newTestDB(t) // Any statement fails the test: none is expected.

	if err := db.DeleteOrphans(t.Context(), &testSharedParent{}); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteOrphansWithoutRelations(t *testing.T) {
	t.Parallel()

	db, _ := newTestDB(t)

	if err := db.DeleteOrphans(t.Context(), &testNote{}); err != nil {
		t.Fatal(err)
	}
}

// TestDeleteOrphansOneStatementPerForeignKey checks that the same table related
// through two foreign keys is swept once for each.
func TestDeleteOrphansOneStatementPerForeignKey(t *testing.T) {
	t.Parallel()

	db, mock := newTestDB(t)

	mock.ExpectExec(
		`DELETE FROM "test_label" WHERE "parent_uuid" NOT IN (SELECT "uuid" FROM "test_two_keys_parent")`,
	).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(
		`DELETE FROM "test_label" WHERE "other_uuid" NOT IN (SELECT "uuid" FROM "test_two_keys_parent")`,
	).WillReturnResult(sqlmock.NewResult(0, 0))

	if err := db.DeleteOrphans(t.Context(), &testTwoKeysParent{}); err != nil {
		t.Fatal(err)
	}
}

// TestDeleteOrphansRejectsMissingForeignKey checks that a relation without a
// foreign key is reported instead of building a statement on an empty column.
func TestDeleteOrphansRejectsMissingForeignKey(t *testing.T) {
	t.Parallel()

	db, _ := newTestDB(t)

	err := db.DeleteOrphans(t.Context(), &testNoKeyParent{})
	if err == nil || !strings.Contains(err.Error(), "test_label") {
		t.Fatalf("got error %v, want one that names test_label", err)
	}
}
