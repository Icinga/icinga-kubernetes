package v1

import (
	"github.com/google/uuid"
	"github.com/icinga/icinga-go-library/types"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
)

var NameSpaceKubernetes = uuid.MustParse("3f249403-2bb0-428f-8e91-504d1fd7ddb6")

type Resource interface {
	kmetav1.Object
	Obtain(k8s kmetav1.Object)
}

type Meta struct {
	Uuid            types.UUID
	Uid             ktypes.UID
	Namespace       string
	Name            string
	ResourceVersion string
	Created         types.UnixMilli
}

func (m *Meta) ObtainMeta(k8s kmetav1.Object) {
	m.Uuid = EnsureUUID(k8s.GetUID())
	m.Uid = k8s.GetUID()
	m.Namespace = k8s.GetNamespace()
	m.Name = k8s.GetName()
	m.ResourceVersion = k8s.GetResourceVersion()
	m.Created = types.UnixMilli(k8s.GetCreationTimestamp().Time)
}

func (m *Meta) GetNamespace() string                           { return m.Namespace }
func (m *Meta) SetNamespace(string)                            { panic("Not expected to be called") }
func (m *Meta) GetName() string                                { return m.Name }
func (m *Meta) SetName(string)                                 { panic("Not expected to be called") }
func (m *Meta) GetGenerateName() string                        { panic("Not expected to be called") }
func (m *Meta) SetGenerateName(string)                         { panic("Not expected to be called") }
func (m *Meta) GetUID() ktypes.UID                             { return m.Uid }
func (m *Meta) SetUID(ktypes.UID)                              { panic("Not expected to be called") }
func (m *Meta) GetResourceVersion() string                     { return m.ResourceVersion }
func (m *Meta) SetResourceVersion(string)                      { panic("Not expected to be called") }
func (m *Meta) GetGeneration() int64                           { panic("Not expected to be called") }
func (m *Meta) SetGeneration(int64)                            { panic("Not expected to be called") }
func (m *Meta) GetSelfLink() string                            { panic("Not expected to be called") }
func (m *Meta) SetSelfLink(string)                             { panic("Not expected to be called") }
func (m *Meta) GetCreationTimestamp() kmetav1.Time             { return kmetav1.NewTime(m.Created.Time()) }
func (m *Meta) SetCreationTimestamp(kmetav1.Time)              { panic("Not expected to be called") }
func (m *Meta) GetDeletionTimestamp() *kmetav1.Time            { panic("Not expected to be called") }
func (m *Meta) SetDeletionTimestamp(*kmetav1.Time)             { panic("Not expected to be called") }
func (m *Meta) GetDeletionGracePeriodSeconds() *int64          { panic("Not expected to be called") }
func (m *Meta) SetDeletionGracePeriodSeconds(*int64)           { panic("Not expected to be called") }
func (m *Meta) GetLabels() map[string]string                   { panic("Not expected to be called") }
func (m *Meta) SetLabels(map[string]string)                    { panic("Not expected to be called") }
func (m *Meta) GetAnnotations() map[string]string              { panic("Not expected to be called") }
func (m *Meta) SetAnnotations(_ map[string]string)             { panic("Not expected to be called") }
func (m *Meta) GetFinalizers() []string                        { panic("Not expected to be called") }
func (m *Meta) SetFinalizers([]string)                         { panic("Not expected to be called") }
func (m *Meta) GetOwnerReferences() []kmetav1.OwnerReference   { panic("Not expected to be called") }
func (m *Meta) SetOwnerReferences([]kmetav1.OwnerReference)    { panic("Not expected to be called") }
func (m *Meta) GetManagedFields() []kmetav1.ManagedFieldsEntry { panic("Not expected to be called") }
func (m *Meta) SetManagedFields([]kmetav1.ManagedFieldsEntry)  { panic("Not expected to be called") }

func EnsureUUID(uid ktypes.UID) types.UUID {
	if id, err := uuid.Parse(string(uid)); err == nil {
		return types.UUID{UUID: id}
	}

	return types.UUID{UUID: uuid.NewSHA1(NameSpaceKubernetes, []byte(uid))}
}

func NewUUID(space types.UUID, data string) types.UUID {
	return types.UUID{UUID: uuid.NewSHA1(space.UUID, []byte(data))}
}

// Assert interface compliance.
var (
	_ kmetav1.Object = (*Meta)(nil)
)
