package schema

import (
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/common"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
)

type kmetaWithoutNamespace struct {
	common.IdMeta
	common.CanonicalMeta
	Uid             ktypes.UID
	Name            string
	ResourceVersion string
	Created         types.UnixMilli
}

func (m *kmetaWithoutNamespace) Fingerprint() database.Fingerprinter {
	return m
}

func (m *kmetaWithoutNamespace) Obtain(kobject kmetav1.Object) {
	m.Uid = kobject.GetUID()
	m.Name = kobject.GetName()
	m.ResourceVersion = kobject.GetResourceVersion()
	m.Created = types.UnixMilli(kobject.GetCreationTimestamp().Time)
}

func (m *kmetaWithoutNamespace) GetNamespace() string       { return "" }
func (m *kmetaWithoutNamespace) SetNamespace(string)        { panic("Not expected to be called") }
func (m *kmetaWithoutNamespace) GetName() string            { return m.Name }
func (m *kmetaWithoutNamespace) SetName(string)             { panic("Not expected to be called") }
func (m *kmetaWithoutNamespace) GetGenerateName() string    { panic("Not expected to be called") }
func (m *kmetaWithoutNamespace) SetGenerateName(string)     { panic("Not expected to be called") }
func (m *kmetaWithoutNamespace) GetUID() ktypes.UID         { return m.Uid }
func (m *kmetaWithoutNamespace) SetUID(ktypes.UID)          { panic("Not expected to be called") }
func (m *kmetaWithoutNamespace) GetResourceVersion() string { return m.ResourceVersion }
func (m *kmetaWithoutNamespace) SetResourceVersion(string)  { panic("Not expected to be called") }
func (m *kmetaWithoutNamespace) GetGeneration() int64       { panic("Not expected to be called") }
func (m *kmetaWithoutNamespace) SetGeneration(int64)        { panic("Not expected to be called") }
func (m *kmetaWithoutNamespace) GetSelfLink() string        { panic("Not expected to be called") }
func (m *kmetaWithoutNamespace) SetSelfLink(string)         { panic("Not expected to be called") }
func (m *kmetaWithoutNamespace) GetCreationTimestamp() kmetav1.Time {
	return kmetav1.NewTime(m.Created.Time())
}
func (m *kmetaWithoutNamespace) SetCreationTimestamp(kmetav1.Time) {
	panic("Not expected to be called")
}
func (m *kmetaWithoutNamespace) GetDeletionTimestamp() *kmetav1.Time {
	panic("Not expected to be called")
}
func (m *kmetaWithoutNamespace) SetDeletionTimestamp(*kmetav1.Time) {
	panic("Not expected to be called")
}
func (m *kmetaWithoutNamespace) GetDeletionGracePeriodSeconds() *int64 {
	panic("Not expected to be called")
}
func (m *kmetaWithoutNamespace) SetDeletionGracePeriodSeconds(*int64) {
	panic("Not expected to be called")
}
func (m *kmetaWithoutNamespace) GetLabels() map[string]string { panic("Not expected to be called") }
func (m *kmetaWithoutNamespace) SetLabels(map[string]string)  { panic("Not expected to be called") }
func (m *kmetaWithoutNamespace) GetAnnotations() map[string]string {
	panic("Not expected to be called")
}
func (m *kmetaWithoutNamespace) SetAnnotations(_ map[string]string) {
	panic("Not expected to be called")
}
func (m *kmetaWithoutNamespace) GetFinalizers() []string { panic("Not expected to be called") }
func (m *kmetaWithoutNamespace) SetFinalizers([]string)  { panic("Not expected to be called") }
func (m *kmetaWithoutNamespace) GetOwnerReferences() []kmetav1.OwnerReference {
	panic("Not expected to be called")
}
func (m *kmetaWithoutNamespace) SetOwnerReferences([]kmetav1.OwnerReference) {
	panic("Not expected to be called")
}
func (m *kmetaWithoutNamespace) GetManagedFields() []kmetav1.ManagedFieldsEntry {
	panic("Not expected to be called")
}
func (m *kmetaWithoutNamespace) SetManagedFields([]kmetav1.ManagedFieldsEntry) {
	panic("Not expected to be called")
}

type kmetaWithNamespace struct {
	kmetaWithoutNamespace
	Namespace string
}

func (m *kmetaWithNamespace) GetNamespace() string { return m.Namespace }

func (m *kmetaWithNamespace) Fingerprint() database.Fingerprinter {
	return m
}

func (m *kmetaWithNamespace) Obtain(kobject kmetav1.Object) {
	m.kmetaWithoutNamespace.Obtain(kobject)

	m.Namespace = kobject.GetNamespace()
}

// Assert interface compliance.
var (
	_ kmetav1.Object = (*kmetaWithoutNamespace)(nil)
	_ kmetav1.Object = (*kmetaWithNamespace)(nil)
)
