package database

import (
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/strcase"
	"github.com/icinga/icinga-go-library/types"
)

// TableName returns the table of t.
func TableName(t any) string {
	if tn, ok := t.(database.TableNamer); ok {
		return tn.TableName()
	}

	if s, ok := t.(string); ok {
		return s
	}

	return strcase.Snake(types.Name(t))
}
