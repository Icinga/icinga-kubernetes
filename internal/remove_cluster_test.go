package internal

import (
	"testing"

	"github.com/icinga/icinga-kubernetes/pkg/database"
	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
)

func TestClusterRemovalContainerExceptions(t *testing.T) {
	wantTables := map[string]struct{}{
		"container":         {},
		"init_container":    {},
		"sidecar_container": {},
	}

	containers := clusterRemovalContainerTypes()
	if len(containers) != len(wantTables) {
		t.Fatalf("cluster removal container types: got %d, want %d", len(containers), len(wantTables))
	}

	for _, container := range containers {
		table := database.TableName(container)

		if _, ok := wantTables[table]; !ok {
			t.Fatalf("unexpected cluster removal container table %q", table)
		}

		if _, ok := container.(database.HasRelations); !ok {
			t.Fatalf("%s does not implement database.HasRelations", table)
		}

		delete(wantTables, table)
	}

	if len(wantTables) != 0 {
		t.Fatalf("missing cluster removal container tables: %v", wantTables)
	}

	podExceptions := map[string]struct{}{
		"container":         {},
		"init_container":    {},
		"sidecar_container": {},
	}

	for _, relation := range (&schemav1.Pod{}).Relations() {
		table := relation.TableName()

		if _, ok := podExceptions[table]; !ok {
			continue
		}

		if relation.CascadeDelete() {
			t.Fatalf(
				"pod relation %q is cascading; explicit cluster-removal handling is no longer justified",
				table,
			)
		}

		delete(podExceptions, table)
	}

	if len(podExceptions) != 0 {
		t.Fatalf("missing non-cascading Pod container relations: %v", podExceptions)
	}

	metricRelation := clusterRemovalContainerMetricRelation()

	if got := metricRelation.TableName(); got != "prometheus_container_metric" {
		t.Fatalf(
			"container metric table: got %q, want %q",
			got,
			"prometheus_container_metric",
		)
	}

	if got := metricRelation.ForeignKey(); got != "container_uuid" {
		t.Fatalf(
			"container metric foreign key: got %q, want %q",
			got,
			"container_uuid",
		)
	}

	for _, container := range clusterRemovalContainerTypes() {
		relations := container.(database.HasRelations).Relations()

		for _, relation := range relations {
			if relation.TableName() == metricRelation.TableName() {
				t.Fatalf(
					"%s already owns %s; explicit metric cleanup is now duplicate",
					database.TableName(container),
					metricRelation.TableName(),
				)
			}
		}
	}
}
