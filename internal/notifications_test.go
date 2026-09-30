package internal

import (
	"testing"

	"github.com/icinga/icinga-go-library/types"
)

func TestBuildActiveResourceQuery(t *testing.T) {
	query, args, err := BuildActiveResourceQuery("pod", []types.UUID{{}})
	if err != nil {
		t.Fatal(err)
	}

	const want = "SELECT uuid FROM pod WHERE uuid IN (?) AND deleted IS NULL"
	if query != want {
		t.Fatalf("expected %q, got %q", want, query)
	}

	if len(args) != 1 {
		t.Fatalf("expected one query argument, got %d", len(args))
	}
}
