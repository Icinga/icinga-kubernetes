package v1

import (
	"github.com/icinga/icinga-kubernetes/pkg/contracts"
	"github.com/icinga/icinga-kubernetes/pkg/types"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
)

type ResourceMeta struct {
	contracts.Meta
	Uid             ktypes.UID
	Namespace       string
	Name            string
	ResourceVersion string
	Created         types.UnixMilli
}

func (m *ResourceMeta) Fingerprint() contracts.FingerPrinter {
	return m
}

func (m *ResourceMeta) ObtainMeta(k8s kmetav1.Object) {
	m.Uid = k8s.GetUID()
	m.Namespace = k8s.GetNamespace()
	m.Name = k8s.GetName()
	m.ResourceVersion = k8s.GetResourceVersion()
	m.Created = types.UnixMilli(k8s.GetCreationTimestamp().Time)
}

func (m *ResourceMeta) GetNamespace() string                  { return m.Namespace }
func (m *ResourceMeta) SetNamespace(string)                   { panic("Not expected to be called") }
func (m *ResourceMeta) GetName() string                       { return m.Name }
func (m *ResourceMeta) SetName(string)                        { panic("Not expected to be called") }
func (m *ResourceMeta) GetGenerateName() string               { panic("Not expected to be called") }
func (m *ResourceMeta) SetGenerateName(string)                { panic("Not expected to be called") }
func (m *ResourceMeta) GetUID() ktypes.UID                    { return m.Uid }
func (m *ResourceMeta) SetUID(ktypes.UID)                     { panic("Not expected to be called") }
func (m *ResourceMeta) GetResourceVersion() string            { return m.ResourceVersion }
func (m *ResourceMeta) SetResourceVersion(string)             { panic("Not expected to be called") }
func (m *ResourceMeta) GetGeneration() int64                  { panic("Not expected to be called") }
func (m *ResourceMeta) SetGeneration(int64)                   { panic("Not expected to be called") }
func (m *ResourceMeta) GetSelfLink() string                   { panic("Not expected to be called") }
func (m *ResourceMeta) SetSelfLink(string)                    { panic("Not expected to be called") }
func (m *ResourceMeta) GetCreationTimestamp() kmetav1.Time    { return kmetav1.NewTime(m.Created.Time()) }
func (m *ResourceMeta) SetCreationTimestamp(kmetav1.Time)     { panic("Not expected to be called") }
func (m *ResourceMeta) GetDeletionTimestamp() *kmetav1.Time   { panic("Not expected to be called") }
func (m *ResourceMeta) SetDeletionTimestamp(*kmetav1.Time)    { panic("Not expected to be called") }
func (m *ResourceMeta) GetDeletionGracePeriodSeconds() *int64 { panic("Not expected to be called") }
func (m *ResourceMeta) SetDeletionGracePeriodSeconds(*int64)  { panic("Not expected to be called") }
func (m *ResourceMeta) GetLabels() map[string]string          { panic("Not expected to be called") }
func (m *ResourceMeta) SetLabels(map[string]string)           { panic("Not expected to be called") }
func (m *ResourceMeta) GetAnnotations() map[string]string     { panic("Not expected to be called") }
func (m *ResourceMeta) SetAnnotations(_ map[string]string)    { panic("Not expected to be called") }
func (m *ResourceMeta) GetFinalizers() []string               { panic("Not expected to be called") }
func (m *ResourceMeta) SetFinalizers([]string)                { panic("Not expected to be called") }
func (m *ResourceMeta) GetOwnerReferences() []kmetav1.OwnerReference {
	panic("Not expected to be called")
}
func (m *ResourceMeta) SetOwnerReferences([]kmetav1.OwnerReference) {
	panic("Not expected to be called")
}
func (m *ResourceMeta) GetManagedFields() []kmetav1.ManagedFieldsEntry {
	panic("Not expected to be called")
}
func (m *ResourceMeta) SetManagedFields([]kmetav1.ManagedFieldsEntry) {
	panic("Not expected to be called")
}

// Assert interface compliance.
var (
	_ kmetav1.Object          = (*ResourceMeta)(nil)
	_ contracts.FingerPrinter = (*ResourceMeta)(nil)
	_ contracts.Checksumer    = (*ResourceMeta)(nil)
	_ contracts.IDer          = (*ResourceMeta)(nil)
	_ contracts.ParentIDer    = (*ResourceMeta)(nil)
	_ contracts.Entity        = (*ResourceMeta)(nil)
)
