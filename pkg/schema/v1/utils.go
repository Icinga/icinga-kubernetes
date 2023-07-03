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

// MustMarshalJSON json encodes the given object.
// TODO: This is just used to generate the checksum of the object properties.
//   - This should no longer be necessary once we have implemented a more sophisticated
//   - method for hashing a structure.
func MustMarshalJSON(v interface{}) []byte {
	b, err := types.MarshalJSON(v)
	if err != nil {
		panic(err)
	}

	return b
}
