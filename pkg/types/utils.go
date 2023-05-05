package types

import (
	"encoding/json"
	"github.com/pkg/errors"
	"math"
	"time"
)

// CantParseFloat64 wraps the given error with the specified string that cannot be parsed into float64.
func CantParseFloat64(err error, s string) error {
	return errors.Wrapf(err, "can't parse %q into float64", s)
}

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

// FromUnixMilli creates and returns a time.Time value
// from the given milliseconds since the Unix epoch ms.
func FromUnixMilli(ms int64) time.Time {
	sec, dec := math.Modf(float64(ms) / 1e3)

	return time.Unix(int64(sec), int64(dec*(1e9)))
}
