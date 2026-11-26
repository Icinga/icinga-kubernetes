package daemon

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/icinga/icinga-go-library/database"
	"github.com/jmoiron/sqlx"
)

func TestCheckSchemaVersionAcceptsLegacyVersion(t *testing.T) {
	conn, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	mock.ExpectQuery(`SELECT "version" FROM kubernetes_schema ORDER BY "id" DESC LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow("0.4.0"))

	db := &database.DB{DB: sqlx.NewDb(conn, database.MySQL)}
	have, err := checkSchemaVersion(t.Context(), db, "v0.5.0")
	if err != nil {
		t.Fatal(err)
	}
	if have != "v0.4.0" {
		t.Fatalf("schema version = %q, want v0.4.0", have)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
