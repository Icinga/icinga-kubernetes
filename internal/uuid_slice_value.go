package internal

import (
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/icinga/icinga-go-library/types"
)

// UuidSliceValue is a pflag.Value that collects UUIDs from a flag that may be
// repeated and may hold a comma-separated list. Invalid UUIDs are rejected
// while the flags are parsed.
type UuidSliceValue struct {
	values []types.UUID
}

// Set implements pflag.Value. It parses val as a comma-separated list of UUIDs
// and appends those not yet collected by this or previous calls.
func (s *UuidSliceValue) Set(val string) error {
	for uuidStr := range strings.SplitSeq(val, ",") {
		u, err := uuid.Parse(strings.TrimSpace(uuidStr))
		if err != nil {
			return err
		}

		uu := types.UUID{UUID: u}

		if !slices.Contains(s.values, uu) {
			s.values = append(s.values, uu)
		}
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
