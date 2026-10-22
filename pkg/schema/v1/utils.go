package v1

import (
	"reflect"

	"github.com/icinga/icinga-go-library/types"
	"golang.org/x/exp/constraints"
	kappsv1 "k8s.io/api/apps/v1"
	kbatchv1 "k8s.io/api/batch/v1"
	kcorev1 "k8s.io/api/core/v1"
	keventsv1 "k8s.io/api/events/v1"
	knetworkingv1 "k8s.io/api/networking/v1"
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

// yamlEncoder is built once so that MarshalYaml doesn't set up a scheme and
// codec on every call. Encoders are safe for concurrent use. A schema type from
// a group not registered here gets empty YAML, since MarshalYaml ignores errors.
var yamlEncoder = func() kruntime.Encoder {
	scheme := kruntime.NewScheme()

	_ = kcorev1.AddToScheme(scheme)
	_ = kappsv1.AddToScheme(scheme)
	_ = kbatchv1.AddToScheme(scheme)
	_ = keventsv1.AddToScheme(scheme)
	_ = knetworkingv1.AddToScheme(scheme)

	return kserializer.NewCodecFactory(scheme).EncoderForVersion(
		kjson.NewSerializerWithOptions(kjson.DefaultMetaFactory, scheme, scheme, kjson.SerializerOptions{Yaml: true}),
		kschema.GroupVersions{
			kcorev1.SchemeGroupVersion,
			kappsv1.SchemeGroupVersion,
			kbatchv1.SchemeGroupVersion,
			keventsv1.SchemeGroupVersion,
			knetworkingv1.SchemeGroupVersion,
		},
	)
}()

// MarshalYaml encodes obj as YAML, including its apiVersion and kind.
// Encoding errors are ignored and yield empty YAML.
func MarshalYaml(obj kruntime.Object) string {
	output, _ := kruntime.Encode(yamlEncoder, obj)

	return string(output)
}

func MaxInt[T constraints.Integer](x, y T) T {
	if x > y {
		return x
	}

	return y
}
