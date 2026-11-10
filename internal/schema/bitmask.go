package schema

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strconv"

	"golang.org/x/exp/constraints"
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
func (b *Bitmask[T]) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		b.bitmask = 0
	case int64:
		b.bitmask = T(v)
	case []byte:
		i, err := strconv.ParseInt(string(v), 10, 64)
		if err != nil {
			return err
		}

		b.bitmask = T(i)
	default:
		return fmt.Errorf("cannot scan %T into Bitmask", src)
	}

	return nil
}

// Value implements the driver.Valuer interface.
func (b Bitmask[T]) Value() (driver.Value, error) {
	if b.bitmask == 0 {
		return nil, nil
	}

	return int64(b.bitmask), nil
}

// Assert interface compliance.
var (
	_ sql.Scanner   = (*Bitmask[byte])(nil)
	_ driver.Valuer = (*Bitmask[byte])(nil)
)
