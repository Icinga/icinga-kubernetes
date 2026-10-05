package v1

import (
	"testing"

	"github.com/icinga/icinga-kubernetes/pkg/database"
)

func TestDeletedPodTrackingIsMonotonic(t *testing.T) {
	id := EnsureUUID("9f1c6a4e-0e0a-4e2b-9a5f-000000000101")
	other := EnsureUUID("9f1c6a4e-0e0a-4e2b-9a5f-000000000102")

	delete(deletedPodIds, id.String())
	delete(deletedPodIds, other.String())

	t.Cleanup(func() {
		delete(deletedPodIds, id.String())
		delete(deletedPodIds, other.String())
	})

	if podWasDeleted(id) {
		t.Fatal("pod must not start as deleted")
	}
	if !markPodDeleted(id) {
		t.Fatal("first deletion must be accepted")
	}
	if !podWasDeleted(id) {
		t.Fatal("pod deletion must remain recorded")
	}
	if markPodDeleted(id) {
		t.Fatal("duplicate deletion must not be accepted as new")
	}
	if podWasDeleted(other) {
		t.Fatal("different Kubernetes UID must remain independent")
	}
}

func TestBuildContainerLogWarmupQueryExcludesDeletedPods(t *testing.T) {
	const base = "SELECT pod_uuid, container_uuid, logs, last_update FROM container_log"
	const want = base + " WHERE EXISTS (SELECT 1 FROM pod WHERE pod.uuid=container_log.pod_uuid AND pod.deleted IS NULL)"

	if got := buildContainerLogWarmupQuery(base); got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestPodContainerRelationsArePurgeOnly(t *testing.T) {
	want := map[string]bool{
		"container":         false,
		"init_container":    false,
		"sidecar_container": false,
	}

	for _, relation := range (&Pod{}).Relations() {
		if _, ok := want[relation.TableName()]; !ok {
			continue
		}

		if relation.CascadeDelete() {
			t.Fatalf("%s must remain excluded from ordinary Pod cascading deletion", relation.TableName())
		}

		if relation.ForeignKey() != "pod_uuid" {
			t.Fatalf("%s foreign key: got %q, want %q", relation.TableName(), relation.ForeignKey(), "pod_uuid")
		}

		purge, ok := relation.(interface{ CascadePurge() bool })
		if !ok || !purge.CascadePurge() {
			t.Fatalf("%s must be included during Pod tombstone purge", relation.TableName())
		}

		want[relation.TableName()] = true
	}

	for table, found := range want {
		if !found {
			t.Fatalf("missing Pod container relation %s", table)
		}
	}
}

func TestContainerMetricRelationIsPurgeOnly(t *testing.T) {
	cases := []struct {
		name      string
		relations []database.Relation
	}{
		{name: "container-common", relations: (&ContainerCommon{}).Relations()},
		{name: "container", relations: (&Container{}).Relations()},
		{name: "init-container", relations: (&InitContainer{}).Relations()},
		{name: "sidecar-container", relations: (&SidecarContainer{}).Relations()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			found := 0

			for _, relation := range tc.relations {
				if relation.TableName() != "prometheus_container_metric" {
					continue
				}

				found++

				if relation.ForeignKey() != "container_uuid" {
					t.Fatalf(
						"foreign key: got %q, want %q",
						relation.ForeignKey(),
						"container_uuid",
					)
				}

				if relation.CascadeDelete() {
					t.Fatal(
						"container metrics must remain excluded from ordinary cascading deletion",
					)
				}

				purge, ok := relation.(interface{ CascadePurge() bool })
				if !ok || !purge.CascadePurge() {
					t.Fatal(
						"container metrics must participate in physical tombstone purge",
					)
				}
			}

			if found != 1 {
				t.Fatalf(
					"prometheus_container_metric relations: got %d, want 1",
					found,
				)
			}
		})
	}
}
