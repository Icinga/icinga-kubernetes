package daemon

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/icinga/icinga-go-library/database"
	k8schema "github.com/icinga/icinga-kubernetes/schema"
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

func TestApplySocketDiscoveryMigrationAfterDuplicateColumn(t *testing.T) {
	for _, test := range []struct {
		name               string
		matchingColumns    int
		wantMigrationError bool
	}{
		{name: "both definitions match", matchingColumns: 2},
		{name: "one definition differs", matchingColumns: 1, wantMigrationError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			conn, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = conn.Close() })

			duplicate := &mysql.MySQLError{Number: 1060, Message: "duplicate column name"}
			mock.ExpectExec(`^ALTER TABLE kubernetes_instance ADD COLUMN`).WillReturnError(duplicate)
			mock.ExpectQuery(`^SELECT COUNT\(\*\) FROM INFORMATION_SCHEMA.COLUMNS`).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(test.matchingColumns))
			if !test.wantMigrationError {
				mock.ExpectExec(`^INSERT INTO kubernetes_schema`).WillReturnResult(sqlmock.NewResult(1, 1))
			}

			db := &database.DB{DB: sqlx.NewDb(conn, database.MySQL)}
			err = applySchemaFile(t.Context(), db, k8schema.MySQLSchemaDir, "mysql/upgrades/0.5.1.sql")
			if test.wantMigrationError {
				if !errors.Is(err, duplicate) {
					t.Fatalf("migration error = %v, want duplicate column error", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
