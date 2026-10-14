package database

import (
	"testing"
)

func TestBuildSoftDeleteStmt(t *testing.T) {
	db := &Database{Quoter: &Quoter{quoteCharacter: "`"}}

	got := db.BuildSoftDeleteStmt("pod")
	want := "UPDATE `pod` SET `deleted` = UNIX_TIMESTAMP() * 1000 WHERE uuid IN (?) AND `deleted` IS NULL"

	if got != want {
		t.Fatalf("unexpected soft-delete statement: got %q, want %q", got, want)
	}
}
