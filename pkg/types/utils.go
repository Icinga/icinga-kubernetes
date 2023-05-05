package types

import (
	"encoding/json"
	"github.com/pkg/errors"
)

// CantParseUint64 wraps the given error with the specified string that cannot be parsed into uint64.
func CantParseUint64(err error, s string) error {
	return errors.Wrapf(err, "can't parse %q into uint64", s)
}

// MarshalJSON calls json.Marshal and wraps any resulting errors.
func MarshalJSON(v interface{}) ([]byte, error) {
	b, err := json.Marshal(v)

	return b, errors.Wrapf(err, "can't marshal JSON from %T", v)
}

// UnmarshalJSON calls json.Unmarshal and wraps any resulting errors.
func UnmarshalJSON(data []byte, v interface{}) error {
	return errors.Wrapf(json.Unmarshal(data, v), "can't unmarshal JSON into %T", v)
}
