package v1

import (
	"github.com/icinga/icinga-kubernetes/pkg/types"
	"reflect"
)

func MarshalFirstNonNilStructFieldToJSON(i any) (string, string, error) {
	v := reflect.ValueOf(i)
	for _, field := range reflect.VisibleFields(v.Type()) {
		if v.FieldByIndex(field.Index).IsNil() {
			continue
		}
		jsn, err := types.MarshalJSON(v.FieldByIndex(field.Index).Interface())
		if err != nil {
			return "", "", err
		}

		return field.Name, string(jsn), nil
	}

	return "", "", nil
}
