package kdatabase

import (
	"context"
	"reflect"

	"github.com/icinga/icinga-go-library/database"
)

type Relation interface {
	ForeignKey() string
	SetForeignKey(fk string)
	CascadeDelete() bool
	WithoutCascadeDelete()
	ReplaceOnUpsert() bool
	WithReplaceOnUpsert()
	StreamInto(context.Context, chan database.Entity) error
	// Entities returns the related entities that StreamInto sends.
	Entities() []database.Entity
	TableName() string
	// NewEntity returns a pointer to a new zero value of the related entity
	// type, even if the related entities are pointers themselves.
	NewEntity() database.Entity
}

// HasRelations is implemented by entities whose related entities are upserted
// and deleted along with them when cascading.
//
// DeleteStreamedCascading calls Relations on zero values, so it must return the
// same relations regardless of the entity's field values. Related entities that
// implement HasRelations themselves are deleted by their own ids, which requires
// a uuid column in their table. DeleteStreamedCascading sets up the whole cascade
// before deleting anything, so relations must not form a cycle.
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

// WithReplaceOnUpsert makes UpsertStreamedCascading replace all rows of the
// relation that reference an upserted entity, which deletes the rows the entity
// no longer has. Use it only for rows that the entity alone determines, as any
// other row referencing the entity is deleted, too.
func WithReplaceOnUpsert() RelationOption {
	return func(r Relation) {
		r.WithReplaceOnUpsert()
	}
}

type relation[T database.Entity] struct {
	foreignKey           string
	withoutCascadeDelete bool
	replaceOnUpsert      bool
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

func (r *relation[T]) ReplaceOnUpsert() bool {
	return r.replaceOnUpsert
}

func (r *relation[T]) WithReplaceOnUpsert() {
	r.replaceOnUpsert = true
}

func (r *relation[T]) TableName() string {
	return database.TableName(r.NewEntity())
}

func (r *relation[T]) NewEntity() database.Entity {
	t := reflect.TypeFor[T]()
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	return reflect.New(t).Interface().(database.Entity)
}

type hasMany[T database.Entity] struct {
	relation[T]
	entities []T
}

func HasMany[T database.Entity](entities []T, options ...RelationOption) Relation {
	r := &hasMany[T]{entities: entities}

	for _, o := range options {
		o(r)
	}

	return r
}

func (r *hasMany[T]) Entities() []database.Entity {
	entities := make([]database.Entity, 0, len(r.entities))
	for _, entity := range r.entities {
		entities = append(entities, entity)
	}

	return entities
}

func (r *hasMany[T]) StreamInto(ctx context.Context, ch chan database.Entity) error {
	for _, entity := range r.entities {
		select {
		case ch <- entity:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

// comparableEntity constrains the type of the entity related by HasOne. It
// must be comparable, as HasOne skips an entity that equals the zero value of
// its type, and implement database.Entity, as StreamInto sends it into the
// stream that UpsertStreamedCascading writes to the database.
type comparableEntity interface {
	comparable
	database.Entity
}

type hasOne[T comparableEntity] struct {
	relation[T]
	entity T
}

func HasOne[T comparableEntity](entity T, options ...RelationOption) Relation {
	r := &hasOne[T]{entity: entity}

	for _, o := range options {
		o(r)
	}

	return r
}

func (r *hasOne[T]) Entities() []database.Entity {
	if r.entity == Zero[T]() {
		return nil
	}

	return []database.Entity{r.entity}
}

func (r *hasOne[T]) StreamInto(ctx context.Context, ch chan database.Entity) error {
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
