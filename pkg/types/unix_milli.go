package types

import (
	"database/sql"
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"strconv"
	"time"
)

// UnixMilli is a nullable millisecond UNIX timestamp in databases and JSON.
type UnixMilli time.Time

// Time returns the time.Time conversion of UnixMilli.
func (t UnixMilli) Time() time.Time {
	return time.Time(t)
}

// MarshalJSON implements the json.Marshaler interface.
// Marshals to milliseconds. Supports JSON null.
func (t UnixMilli) MarshalJSON() ([]byte, error) {
	if time.Time(t).IsZero() {
		return nil, nil
	}

	return []byte(strconv.FormatInt(time.Time(t).UnixMilli(), 10)), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (t *UnixMilli) UnmarshalText(text []byte) error {
	i, err := strToInt(string(text))
	if err != nil {
		return err
	}

	*t = UnixMilli(FromUnixMilli(i))

	return nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// Unmarshals from milliseconds. Supports JSON null.
func (t *UnixMilli) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || len(data) == 0 {
		return nil
	}

	i, err := strToInt(string(data))
	if err != nil {
		return err
	}

	*t = UnixMilli(FromUnixMilli(i))

	return nil
}

// Scan implements the sql.Scanner interface.
// Scans from milliseconds. Supports SQL NULL.
func (t *UnixMilli) Scan(src interface{}) error {
	if src == nil {
		return nil
	}

	i, err := strToInt(string(src.([]byte)))
	if err != nil {
		return err
	}

	*t = UnixMilli(FromUnixMilli(i))

	return nil
}

// Value implements the driver.Valuer interface.
// Returns milliseconds. Supports SQL NULL.
func (t UnixMilli) Value() (driver.Value, error) {
	if t.Time().IsZero() {
		return nil, nil
	}

	return t.Time().UnixMilli(), nil
}

func strToInt(s string) (int64, error) {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, CantParseInt64(err, s)
	}

	return i, nil
}

// Assert interface compliance.
var (
	_ json.Marshaler           = (*UnixMilli)(nil)
	_ encoding.TextUnmarshaler = (*UnixMilli)(nil)
	_ json.Unmarshaler         = (*UnixMilli)(nil)
	_ sql.Scanner              = (*UnixMilli)(nil)
	_ driver.Valuer            = (*UnixMilli)(nil)
)