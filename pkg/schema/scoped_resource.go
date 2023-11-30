package schema

import (
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-kubernetes/pkg/contracts"
)

type ScopedResource struct {
	contracts.Resource
	scope interface{}
}

func (r *ScopedResource) Scope() interface{} {
	return r.scope
}

func (r *ScopedResource) TableName() string {
	return database.TableName(r.Resource)
}

func NewScopedResource(resource contracts.Resource, scope interface{}) *ScopedResource {
	return &ScopedResource{
		Resource: resource,
		scope:    scope,
	}
}
