package contracts

import (
	"github.com/icinga/icinga-go-library/database"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KEnvelope carries identity information about a Kubernetes resource change to be synchronized.
type KEnvelope interface {
	entity
	KUpsert(kobject kmetav1.Object) KUpsert
	KDelete() KDelete
}

// KUpsert carries identity information and the added or updated Kubernetes resource to be synchronized.
type KUpsert interface {
	entity
	KObject() kmetav1.Object
}

// KDelete carries identity information about a deleted Kubernetes resource to be synchronized.
type KDelete interface {
	entity
}

// Resource represents principal entities synchronized from Kubernetes resources to the Icinga Kubernetes database.
type Resource interface {
	entity
	database.Fingerprinter
	v1.Object
	Obtain(kobject v1.Object)
}

type entity interface {
	database.IDer
	GetCanonicalName() string
	SetCanonicalName(string)
}
