package database

import "testing"

type relationPointerTableNameFixture struct {
	Values []string
}

func (relationPointerTableNameFixture) TableName() string {
	return "relation_pointer_table_name_fixture"
}

type relationValueTableNameFixture struct{}

func (relationValueTableNameFixture) TableName() string {
	return "relation_value_table_name_fixture"
}

func TestRelationTableName(t *testing.T) {
	t.Run("pointer entity with value receiver", func(t *testing.T) {
		relation := HasMany([]*relationPointerTableNameFixture(nil))

		if got := relation.TableName(); got != "relation_pointer_table_name_fixture" {
			t.Fatalf("table name: got %q, want %q", got, "relation_pointer_table_name_fixture")
		}

		entity, ok := relation.NewEntity().(*relationPointerTableNameFixture)
		if !ok || entity == nil {
			t.Fatalf("NewEntity: got %T, want non-nil *relationPointerTableNameFixture", relation.NewEntity())
		}
	})

	t.Run("value entity remains unchanged", func(t *testing.T) {
		relation := HasMany([]relationValueTableNameFixture(nil))

		if got := relation.TableName(); got != "relation_value_table_name_fixture" {
			t.Fatalf("table name: got %q, want %q", got, "relation_value_table_name_fixture")
		}
	})
}
