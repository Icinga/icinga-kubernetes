package database

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

func BuildUpsertStmt(subject interface{}) string {
	v := reflect.ValueOf(subject)
	t := v.Type()

	var columns []string
	for i := 0; i < v.NumField(); i++ {
		tag := t.Field(i).Tag.Get("db")
		columns = append(columns, tag)
	}

	tableName := ToSnakeCase(reflect.TypeOf(subject).Name())
	updateSet := make([]string, len(columns))
	for i, col := range columns {
		updateSet[i] = fmt.Sprintf("%s = VALUES(%s)", col, col)
	}

	upsert := `INSERT INTO %s (%s) VALUES (:%s) ON DUPLICATE KEY UPDATE %s`

	var1 := strings.Join(columns, ", ")
	var2 := strings.Join(columns, ", :")
	updateSetStr := strings.Join(updateSet, ", ")

	return fmt.Sprintf(
		upsert,
		tableName,
		var1,
		var2,
		updateSetStr,
	)
}

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

func ToSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}
