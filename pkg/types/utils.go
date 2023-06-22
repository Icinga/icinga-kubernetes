package types

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"golang.org/x/exp/constraints"
	"math"
	"strings"
	"time"
)

// CantDecodeHex wraps the given error with the given string that cannot be hex-decoded.
func CantDecodeHex(err error, s string) error {
	return errors.Wrapf(err, "can't decode hex %q", s)
}

// CantParseFloat64 wraps the given error with the specified string that cannot be parsed into float64.
func CantParseFloat64(err error, s string) error {
	return errors.Wrapf(err, "can't parse %q into float64", s)
}

// CantParseInt64 wraps the given error with the specified string that cannot be parsed into int64.
func CantParseInt64(err error, s string) error {
	return errors.Wrapf(err, "can't parse %q into int64", s)
}

// CantParseUint64 wraps the given error with the specified string that cannot be parsed into uint64.
func CantParseUint64(err error, s string) error {
	return errors.Wrapf(err, "can't parse %q into uint64", s)
}

// CantPerformQuery wraps the given error with the specified query that cannot be executed.
func CantPerformQuery(err error, q string) error {
	return errors.Wrapf(err, "can't perform %q", q)
}

// CantUnmarshalYAML wraps the given error with the designated value, which cannot be unmarshalled into.
func CantUnmarshalYAML(err error, v interface{}) error {
	return errors.Wrapf(err, "can't unmarshal YAML into %T", v)
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

// Name returns the declared name of type t.
func Name(t interface{}) string {
	s := strings.TrimLeft(fmt.Sprintf("%T", t), "*")

	return s[strings.LastIndex(s, ".")+1:]
}

// Checksum returns the SHA-1 checksum of the data.
func Checksum(data interface{}) []byte {
	var chksm [sha1.Size]byte

	switch data := data.(type) {
	case string:
		chksm = sha1.Sum([]byte(data))
	case []byte:
		chksm = sha1.Sum(data)
	default:
		panic(fmt.Sprintf("Unable to create checksum for type %T", data))
	}

	return chksm[:]
}

// FromUnixMilli creates and returns a time.Time value
// from the given milliseconds since the Unix epoch ms.
func FromUnixMilli(ms int64) time.Time {
	sec, dec := math.Modf(float64(ms) / 1e3)

	return time.Unix(int64(sec), int64(dec*(1e9)))
}

func MaxInt[T constraints.Integer](x, y T) T {
	if x > y {
		return x
	}

	return y
}
