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
	CascadeSelect() bool
	WithoutCascadeSelect()
	StreamInto(context.Context, chan interface{}) error
	TableName() string
	TypePointer() any
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

func WithoutCascadeSelect() RelationOption {
	return func(r Relation) {
		r.WithoutCascadeSelect()
	}
}

type relation[T any] struct {
	foreignKey           string
	withoutCascadeDelete bool
	withoutCascadeSelect bool
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

func (r *relation[T]) CascadeSelect() bool {
	return !r.withoutCascadeSelect
}

func (r *relation[T]) WithoutCascadeSelect() {
	r.withoutCascadeSelect = true
}

func (r *relation[T]) TableName() string {
	return TableName(*new(T))
}

func (r *relation[T]) TypePointer() any {
	var typ T
	if reflect.ValueOf(typ).Kind() == reflect.Ptr {
		return reflect.New(reflect.TypeOf(typ).Elem()).Interface()
	}

	return &typ
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
