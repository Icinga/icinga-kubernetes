package v1

import (
	"testing"

	"github.com/icinga/icinga-kubernetes/pkg/kdatabase"
)

// TestRelationsForeignKeys checks for every kind that its labels, annotations
// and selectors are deleted through the column that references the kind. The
// cascade tests in kdatabase only use test entities, so a kind that loses such
// a foreign key would go unnoticed there.
func TestRelationsForeignKeys(t *testing.T) {
	resource := map[string]string{"label": "resource_uuid", "annotation": "resource_uuid"}

	tests := map[string]struct {
		entity kdatabase.HasRelations
		// Foreign keys by the table of the related entities.
		fks map[string]string
	}{
		"ConfigMap":  {&ConfigMap{}, resource},
		"CronJob":    {&CronJob{}, resource},
		"DaemonSet":  {&DaemonSet{}, resource},
		"Deployment": {&Deployment{}, resource},
		// Endpoint slices store no annotations.
		"EndpointSlice":    {&EndpointSlice{}, map[string]string{"label": "resource_uuid"}},
		"Ingress":          {&Ingress{}, resource},
		"Job":              {&Job{}, resource},
		"Namespace":        {&Namespace{}, resource},
		"Node":             {&Node{}, resource},
		"PersistentVolume": {&PersistentVolume{}, resource},
		"Pod":              {&Pod{}, resource},
		"Pvc":              {&Pvc{}, resource},
		"ReplicaSet":       {&ReplicaSet{}, resource},
		"Secret":           {&Secret{}, resource},
		"Service": {&Service{}, map[string]string{
			"label":            "resource_uuid",
			"annotation":       "resource_uuid",
			"service_selector": "service_uuid",
		}},
		"StatefulSet": {&StatefulSet{}, resource},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// DeleteStreamedCascading calls Relations on zero values as well.
			relations := make(map[string]kdatabase.Relation)
			for _, r := range tt.entity.Relations() {
				relations[r.TableName()] = r
			}

			for table, fk := range tt.fks {
				r, ok := relations[table]
				if !ok {
					t.Errorf("%s: no relation", table)

					continue
				}

				if got := r.ForeignKey(); got != fk {
					t.Errorf("%s: got foreign key %q, want %q", table, got, fk)
				}

				if !r.CascadeDelete() {
					t.Errorf("%s: not deleted by cascade", table)
				}
			}
		})
	}
}
