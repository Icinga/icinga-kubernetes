package schema

import (
	"reflect"

	"github.com/icinga/icinga-go-library/types"
	"github.com/pkg/errors"
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
// codec on every call. Encoders are safe for concurrent use. New schema types
// from other API groups must register their group here, or MarshalYaml panics.
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
// It panics if yamlEncoder doesn't register the API group of obj.
func MarshalYaml(obj kruntime.Object) string {
	output, err := kruntime.Encode(yamlEncoder, obj)
	if err != nil {
		// Encoding fails for types whose API group yamlEncoder doesn't
		// register, which is a bug rather than a runtime condition.
		panic(errors.Wrapf(err, "cannot encode %T as YAML", obj))
	}

	return string(output)
}

func MaxInt[T constraints.Integer](x, y T) T {
	if x > y {
		return x
	}

	return y
}

// LabelsToMap converts a slice of Label structs to a map of label names to label values.
func LabelsToMap(labels []Label) map[string]string {
	labelMap := make(map[string]string, len(labels))
	for _, label := range labels {
		labelMap[label.Name] = label.Value
	}
	return labelMap
}

// Owner is the constraint satisfied by all owner reference schema types.
type Owner interface {
	PodOwner | DeploymentOwner | ReplicaSetOwner | DaemonSetOwner | StatefulSetOwner | JobOwner
}

// OwnersToRelation converts a slice of owner structs to a slice of maps containing the kind and name of each owner.
func OwnersToRelation[T Owner](owners []T) []map[string]string {
	ownerSlice := make([]map[string]string, len(owners))
	for i, owner := range owners {
		var kind, name string
		switch o := any(owner).(type) {
		case PodOwner:
			kind = o.Kind
			name = o.Name
		case DeploymentOwner:
			kind = o.Kind
			name = o.Name
		case ReplicaSetOwner:
			kind = o.Kind
			name = o.Name
		case DaemonSetOwner:
			kind = o.Kind
			name = o.Name
		case StatefulSetOwner:
			kind = o.Kind
			name = o.Name
		case JobOwner:
			kind = o.Kind
			name = o.Name
		default:
			panic("unreachable")
		}

		ownerSlice[i] = map[string]string{
			"kind": kind,
			"name": name,
		}
	}

	return ownerSlice
}
