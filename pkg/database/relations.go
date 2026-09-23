package database

import (
	"context"
	"reflect"
)

type Relation interface {
	ForeignKey() string
	SetForeignKey(fk string)
	CascadeDelete() bool
	WithoutCascadeDelete()
	StreamInto(context.Context, chan any) error
	TableName() string
	// NewEntity returns a pointer to a new zero value of the related entity
	// type, even if the related entities are pointers themselves.
	NewEntity() any
}

// HasRelations is implemented by entities whose related entities are upserted
// and deleted along with them when cascading.
//
// DeleteStreamed calls Relations on zero values, so it must return the same
// relations regardless of the entity's field values. Related entities that
// implement HasRelations themselves are deleted by their own ids, which requires
// a uuid column in their table. DeleteStreamed sets up the whole cascade before
// deleting anything, so relations must not form a cycle.
type HasRelations interface {
	Relations() []Relation
}

type RelationOption func(r Relation)

func WithForeignKey(fk string) RelationOption {
	return func(r Relation) {
		r.SetForeignKey(fk)
	}
}

func WithoutCascadeDelete() RelationOption {
	return func(r Relation) {
		r.WithoutCascadeDelete()
	}
}

type relation[T comparable] struct {
	foreignKey           string
	withoutCascadeDelete bool
}

func (r *relation[T]) ForeignKey() string {
	return r.foreignKey
}

func (r *relation[T]) SetForeignKey(fk string) {
	r.foreignKey = fk
}

func (r *relation[T]) CascadeDelete() bool {
	return !r.withoutCascadeDelete
}

func (r *relation[T]) WithoutCascadeDelete() {
	r.withoutCascadeDelete = true
}

func (r *relation[T]) TableName() string {
	return TableName(*new(T))
}

func (r *relation[T]) NewEntity() any {
	t := reflect.TypeFor[T]()
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	return reflect.New(t).Interface()
}

type hasMany[T comparable] struct {
	relation[T]
	entities []T
}

func HasMany[T comparable](entities []T, options ...RelationOption) Relation {
	r := &hasMany[T]{entities: entities}

	for _, o := range options {
		o(r)
	}

	return r
}

func (r *hasMany[T]) StreamInto(ctx context.Context, ch chan any) error {
	for _, entity := range r.entities {
		select {
		case ch <- entity:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

type hasOne[T comparable] struct {
	relation[T]
	entity T
}

func HasOne[T comparable](entity T, options ...RelationOption) Relation {
	r := &hasOne[T]{entity: entity}

	for _, o := range options {
		o(r)
	}

	return r
}

func (r *hasOne[T]) StreamInto(ctx context.Context, ch chan any) error {
	if r.entity != Zero[T]() {
		select {
		case ch <- r.entity:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

func Zero[T any]() T {
	return *new(T)
}
