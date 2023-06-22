package database

import (
	"context"
)

type Relation interface {
	ForeignKey() string
	SetForeignKey(fk string)
	CascadeDelete() bool
	WithoutCascadeDelete()
	StreamInto(context.Context, chan interface{}) error
	TableName() string
}

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

type relation[T any] struct {
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

type hasMany[T any] struct {
	relation[T]
	entities []T
}

func HasMany[T any](entities []T, options ...RelationOption) Relation {
	r := &hasMany[T]{entities: entities}

	for _, o := range options {
		o(r)
	}

	return r
}

func (r *hasMany[T]) StreamInto(ctx context.Context, ch chan interface{}) error {
	for _, entity := range r.entities {
		select {
		case ch <- entity:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

type hasOne[T any] struct {
	relation[T]
	entity T
}

func HasOne[T any](entity T, options ...RelationOption) Relation {
	r := &hasOne[T]{entity: entity}

	for _, o := range options {
		o(r)
	}

	return r
}

func (r *hasOne[T]) StreamInto(ctx context.Context, ch chan interface{}) error {
	select {
	case ch <- r.entity:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}
