package types

import (
	"database/sql"
	"database/sql/driver"
	"golang.org/x/exp/constraints"
	"strconv"
)

type Bitmask[T constraints.Integer] struct {
	bitmask T
}

func (b Bitmask[T]) Bits() T         { return b.bitmask }
func (b Bitmask[T]) Has(flag T) bool { return b.bitmask&flag != 0 }
func (b *Bitmask[T]) Set(flag T)     { b.bitmask |= flag }
func (b *Bitmask[T]) Clear(flag T)   { b.bitmask &= ^flag }
func (b *Bitmask[T]) Toggle(flag T)  { b.bitmask ^= flag }

// Scan implements the sql.Scanner interface.
func (b *Bitmask[T]) Scan(src interface{}) error {
	i, err := strconv.ParseInt(string(src.([]byte)), 10, 64)
	if err != nil {
		return err
	}

	b.bitmask = T(i)

	return nil
}

// Value implements the driver.Valuer interface.
func (b Bitmask[T]) Value() (driver.Value, error) {
	return int64(b.bitmask), nil
}

// Assert interface compliance.
var (
	_ sql.Scanner   = (*Bitmask[byte])(nil)
	_ driver.Valuer = (*Bitmask[byte])(nil)
)
