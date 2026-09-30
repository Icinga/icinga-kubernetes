package database

import (
	"context"
	"testing"

	ingdatabase "github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/strcase"
	"github.com/icinga/icinga-go-library/types"
	"github.com/jmoiron/sqlx/reflectx"
)

func TestBuildSoftDeleteStmt(t *testing.T) {
	db := &Database{Quoter: &Quoter{quoteCharacter: "`"}}

	got := db.BuildSoftDeleteStmt("pod")
	want := "UPDATE `pod` SET deleted = UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000 WHERE uuid IN (?)"

	if got != want {
		t.Fatalf("unexpected soft-delete statement: got %q, want %q", got, want)
	}
}

func TestPurgeOnlyRelation(t *testing.T) {
	relation := HasMany([]struct{}(nil), WithoutCascadeDelete(), WithCascadePurge())

	if relation.CascadeDelete() {
		t.Fatal("purge-only relation must remain excluded from ordinary cascading deletion")
	}

	purge, ok := relation.(cascadePurgeRelation)
	if !ok || !purge.CascadePurge() {
		t.Fatal("purge-only relation must be included during physical purge")
	}

	if !NewFeatures(withPurgeRelations()).purgeRelations {
		t.Fatal("purge relation feature was not enabled")
	}
}

func TestBuildDeletedPurgeSelectStmt(t *testing.T) {
	mapper := reflectx.NewMapperFunc("db", strcase.Snake)
	db := &Database{
		Quoter:    &Quoter{quoteCharacter: "`"},
		columnMap: ingdatabase.NewColumnMap(mapper),
	}

	got := db.buildDeletedPurgeSelectStmt("pod")
	want := "SELECT `uuid` FROM `pod` WHERE cluster_uuid=:cluster_uuid AND deleted IS NOT NULL AND deleted < :time"

	if got != want {
		t.Fatalf("unexpected purge selection statement: got %q, want %q", got, want)
	}
}

func TestPeriodicPurgeDeletedRejectsZeroRetention(t *testing.T) {
	db := &Database{}

	err := db.PeriodicPurgeDeleted(context.Background(), "pod", types.UUID{}, 0)
	if err == nil {
		t.Fatal("expected zero deleted retention to be rejected")
	}
}
