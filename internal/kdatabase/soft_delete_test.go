package kdatabase

import (
	"testing"

	"github.com/icinga/icinga-go-library/database"
	"github.com/jmoiron/sqlx"
	kcorev1 "k8s.io/api/core/v1"
)

func TestBuildSoftDeleteStmt(t *testing.T) {
	tests := []struct {
		driverName string
		want       string
	}{
		{
			driverName: database.MySQL,
			want:       `UPDATE "pod" SET "deleted" = UNIX_TIMESTAMP() * 1000 WHERE "uuid" IN (?) AND "deleted" IS NULL`,
		},
		{
			driverName: database.PostgreSQL,
			want:       `UPDATE "pod" SET "deleted" = (EXTRACT(EPOCH FROM NOW()) * 1000)::bigint WHERE "uuid" IN (?) AND "deleted" IS NULL`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.driverName, func(t *testing.T) {
			db := &DB{DB: &database.DB{DB: sqlx.NewDb(nil, tt.driverName)}}

			if got := db.BuildSoftDeleteStmt(&kcorev1.Pod{}); got != tt.want {
				t.Fatalf("unexpected soft-delete statement: got %q, want %q", got, tt.want)
			}
		})
	}
}
