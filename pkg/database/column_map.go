package database

import (
	"database/sql/driver"
	"github.com/jmoiron/sqlx/reflectx"
	"reflect"
	"sync"
)

var valuerReflectType = reflect.TypeOf((*driver.Valuer)(nil)).Elem()

type ColumnMap struct {
	mutex  sync.Mutex
	cache  map[reflect.Type][]string
	mapper *reflectx.Mapper
}

func NewColumnMap(mapper *reflectx.Mapper) *ColumnMap {
	return &ColumnMap{
		cache:  make(map[reflect.Type][]string),
		mapper: mapper,
	}
}

func (m *ColumnMap) Columns(subject any) []string {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	t, ok := subject.(reflect.Type)
	if !ok {
		t = reflect.TypeOf(subject)
	}

	columns, ok := m.cache[t]
	if !ok {
		columns = m.getColumns(t)
		m.cache[t] = columns
	}

	return columns
}

func (m *ColumnMap) getColumns(t reflect.Type) []string {
	fields := m.mapper.TypeMap(t).Names
	all := make(map[string]struct{}, len(fields))
	columns := make([]string, 0, len(fields))
	ignores := make(map[string]struct{}, 2)

	for _, f := range fields {
		// if _, ok := f.Zero.Interface().(driver.Valuer); ok {
		// }
		if f.Field.Type.Implements(valuerReflectType) {
			for _, c := range f.Children {
				if c != nil {
					ignores[c.Path] = struct{}{}
				}
			}
		}

		all[f.Path] = struct{}{}
	}

	for column := range all {
		if _, ignore := ignores[column]; !ignore {
			columns = append(columns, column)
		}
	}

	return columns[:len(columns):len(columns)]
}
