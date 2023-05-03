package database

import "context"

// TableNamer implements the TableName method,
// which returns the table of the object.
type TableNamer interface {
	TableName() string // TableName tells the table.
}

type Upserter interface {
	Upsert() interface{}
}

type Relations []Relation

type HasRelations interface {
	Relations() Relations
}

type HasMany[T any] struct {
	Entities    []T
	ForeignKey_ interface{}
}

func (r HasMany[T]) StreamInto(ctx context.Context, ch chan interface{}) error {
	for _, entity := range r.Entities {
		select {
		case ch <- entity:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

func (r HasMany[T]) TableName() string {
	return TableName(*new(T))
}

func (r HasMany[T]) ForeignKey() interface{} {
	return r.ForeignKey_
}

type Relation interface {
	ForeignKey() interface{}
	StreamInto(context.Context, chan interface{}) error
}
