package daemon

import (
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

func TestApplySchemaRejectsVersionsBeforeV040(t *testing.T) {
	for _, version := range []string{"0.2.0", "0.3.0", "v0.3.0"} {
		t.Run(version, func(t *testing.T) {
			conn, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = conn.Close() })

			mock.ExpectQuery(`SELECT 1 FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=?`).
				WithArgs("kubernetes_schema").
				WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
			mock.ExpectQuery(`SELECT "version" FROM kubernetes_schema ORDER BY "id" DESC LIMIT 1`).
				WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(version))

			db := &database.DB{DB: sqlx.NewDb(conn, database.MySQL)}
			logger := logging.NewLogger(zap.NewNop().Sugar(), time.Second)
			err = ApplySchema(t.Context(), db, logger)
			if err == nil || !strings.Contains(err.Error(), "older than the minimum supported version v0.4.0") {
				t.Fatalf("migration error = %v, want unsupported schema version before any DDL", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
