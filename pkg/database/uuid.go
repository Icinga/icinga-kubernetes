package database

import (
	"database/sql/driver"
	"encoding"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/icinga/icinga-go-library/types"
)

// UUID is like uuid.UUID, but marshals itself binarily
// (not like xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx) in SQL context.
type UUID struct {
	uuid.UUID
}

// Value implements driver.Valuer.
func (uuid UUID) Value() (driver.Value, error) {
	return uuid.UUID[:], nil
}

// Assert interface compliance.
var (
	_ encoding.TextUnmarshaler = (*UUID)(nil)
	_ driver.Valuer            = UUID{}
	_ driver.Valuer            = (*UUID)(nil)
)

// UuidSliceValue is a pflag.Value that collects UUIDs from a flag that may be
// repeated and may hold a comma-separated list. Invalid UUIDs are rejected
// while the flags are parsed.
type UuidSliceValue struct {
	values []types.UUID
}

// Set implements pflag.Value. It parses val as a comma-separated list of UUIDs
// and appends them to the values of previous calls.
func (s *UuidSliceValue) Set(val string) error {
	for uuidStr := range strings.SplitSeq(val, ",") {
		u, err := uuid.Parse(uuidStr)
		if err != nil {
			return err
		}

		s.values = append(s.values, types.UUID{UUID: u})
	}

	return nil
}

// Type implements pflag.Value.
func (s *UuidSliceValue) Type() string {
	return "uuidSlice"
}

// String implements pflag.Value.
func (s *UuidSliceValue) String() string {
	var uuidStrs []string
	for _, u := range s.values {
		uuidStrs = append(uuidStrs, u.String())
	}

	if len(uuidStrs) == 0 {
		return ""
	}

	return "[" + strings.Join(uuidStrs, " ") + "]"
}

// Value returns a copy of the parsed UUIDs.
func (s *UuidSliceValue) Value() []types.UUID {
	return slices.Clone(s.values)
}
