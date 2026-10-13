package kdatabase

import (
	"testing"

	kcorev1 "k8s.io/api/core/v1"
)

func TestBuildSoftDeleteStmt(t *testing.T) {
	got := new(DB).BuildSoftDeleteStmt(&kcorev1.Pod{})
	want := `UPDATE "pod" SET "deleted" = UNIX_TIMESTAMP() * 1000 WHERE "uuid" IN (?) AND "deleted" IS NULL`

	if got != want {
		t.Fatalf("unexpected soft-delete statement: got %q, want %q", got, want)
	}
}
