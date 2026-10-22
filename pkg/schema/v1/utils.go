package v1

import (
	"reflect"

	"github.com/icinga/icinga-go-library/types"
	"golang.org/x/exp/constraints"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	kschema "k8s.io/apimachinery/pkg/runtime/schema"
	kserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
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

// MarshalYaml encodes obj as YAML in API version gv, including its apiVersion
// and kind. addToScheme must register the API group of gv. Encoding errors are
// ignored and yield empty YAML.
func MarshalYaml(obj kruntime.Object, gv kschema.GroupVersion, addToScheme func(*kruntime.Scheme) error) string {
	scheme := kruntime.NewScheme()
	_ = addToScheme(scheme)
	codec := kserializer.NewCodecFactory(scheme).EncoderForVersion(
		kjson.NewSerializerWithOptions(kjson.DefaultMetaFactory, scheme, scheme, kjson.SerializerOptions{Yaml: true}),
		gv,
	)
	output, _ := kruntime.Encode(codec, obj)

	return string(output)
}

func MaxInt[T constraints.Integer](x, y T) T {
	if x > y {
		return x
	}

	return y
}
