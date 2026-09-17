# Icinga for Kubernetes Issue #218 — One-Shot Resolution

## The Problem

In a multi-cluster Icinga for Kubernetes setup, several cluster daemons can write into the same database.

For example:

```text
Cluster A daemon ─┐
                  ├──► shared Icinga Kubernetes database
Cluster B daemon ─┘                 │
                                    ▼
                           Icinga Kubernetes Web
```

If Cluster B is permanently decommissioned, its daemon stops writing new data, but its old database state remains.

The result is:

```text
Cluster B decommissioned
        ↓
daemon stops
        ↓
heartbeat becomes stale
        ↓
old Cluster B resources remain in the database
        ↓
Cluster B remains visible in Kubernetes Web
```

Deleting only the row from the `cluster` table is not sufficient because a cluster owns many resources and dependent relation tables.

---

## What We Changed

The resolution was implemented in the Go daemon/database layer.

Runtime source changes:

```text
cmd/icinga-kubernetes/main.go
pkg/database/remove_cluster.go
pkg/database/cluster_removal_inspection.go
pkg/database/cluster_removal_inspection_test.go
```

### `cmd/icinga-kubernetes/main.go`

Added the DB-only administrative interface:

```text
--remove-cluster <UUID>
--confirm-cluster-removal
```

The active-heartbeat safety window is fixed at `5m`.

The removal command runs before normal Kubernetes client initialization, so removal does not require a working kubeconfig or reachable Kubernetes API.

It also verifies that the existing database schema is present and is the supported schema version before allowing removal.

### `pkg/database/cluster_removal_inspection.go`

Added a read-only lifecycle inspection step.

A cluster is classified as:

```text
missing
no_instance
active
stale
```

This separates:

```text
"what does the database currently show?"
```

from:

```text
"should this cluster actually be deleted?"
```

### `pkg/database/remove_cluster.go`

Added the actual cluster-scoped cleanup operation.

`RemoveCluster()` removes Cluster B's resources and dependent rows inside one SQL transaction:

```text
BEGIN
  ↓
dependent rows
  ↓
resource relation rows
  ↓
cluster-owned resources
  ↓
metrics / config / instance state
  ↓
cluster row LAST
  ↓
COMMIT
```

If one of the deletion steps fails:

```text
ROLLBACK
```

so the database is not intentionally left half-cleaned.

### `pkg/database/cluster_removal_inspection_test.go`

Added lifecycle-classification tests, including:

```text
no instance
fresh heartbeat
heartbeat exactly on boundary
stale heartbeat
future heartbeat / clock skew
multiple instance records
```

---

## Safety Rules

A stale heartbeat is **not automatically permission to delete a cluster**.

The implemented policy is:

```text
missing
    → nothing to remove

active
    → REFUSE

no_instance
    → REFUSE

stale
    → inspection only unless explicitly confirmed
```

The confirmation flag does not override the active-cluster protection.

There is intentionally no:

```text
--force-delete-active-cluster
```

style bypass.

This operation is intended for a cluster that has actually been permanently decommissioned and whose Icinga Kubernetes daemon has been stopped.

---

## Real-World Removal Flow

When permanently removing a monitored cluster:

```text
Decide that Cluster B is permanently decommissioned
        ↓
stop its icinga-kubernetes daemon
        ↓
identify Cluster B UUID
        ↓
run removal command without confirmation
        ↓
InspectClusterRemoval()
        ↓
        ├── active      → STOP
        ├── no_instance → STOP
        ├── missing     → nothing to do
        └── stale       → eligible for explicit confirmation
                               ↓
                     run confirmed removal
                               ↓
                       RemoveCluster()
                               ↓
                   transactional DB cleanup
                               ↓
                    inspect Cluster B again
                               ↓
                       require "missing"
                               ↓
                Cluster B disappears from Web
```

### First run — inspection / dry run

Using the same database configuration used by the daemon:

```bash
icinga-kubernetes \
    --remove-cluster <CLUSTER_UUID>
```

For a stale cluster this reports that the cluster is eligible for explicit removal but does not delete anything.

### Confirmed removal

After confirming that the cluster really is permanently decommissioned:

```bash
icinga-kubernetes \
    --remove-cluster <CLUSTER_UUID> \
    --confirm-cluster-removal
```

The command performs the transactional cleanup and then checks the database again.

The final expected lifecycle state is:

```text
missing
```

The built-in `5m` safety window remains in effect for both inspection and confirmed removal. A cluster whose newest heartbeat is within that window is classified as `active` and removal is refused. `--confirm-cluster-removal` does not override that protection.

---

## Why No Kubernetes Web Deletion Code Was Needed

Kubernetes Web already builds its cluster selector from rows in the `cluster` table.

Therefore:

```text
RemoveCluster()
      ↓
Cluster B database state removed
      ↓
Cluster B cluster row removed last
      ↓
next Web query
      ↓
Cluster B no longer exists in selector data
```

The Web module does not need its own duplicate implementation of the cluster deletion graph.

---

## What Was Verified

The implementation was tested first against a disposable copy of the reproduced database and then against the original reproduced environment.

Validation included:

```text
active-cluster refusal
stale-cluster dry run
confirmed stale removal
missing-cluster no-op
transaction rollback on forced SQL failure
other-cluster isolation
full Go test suite
race-enabled Go tests
complete Go build
Web verification
```

The final deep database audit covered:

```text
141 unique checks
0 audit failures
0 Cluster B orphan rows
```

Cluster A remained present, its heartbeat continued advancing and its Web data remained usable.

Cluster B was removed from the database and no longer appeared as a valid Web cluster.

---

## Final Resolution

The problem was resolved by making cluster removal:

```text
explicit
+
cluster-scoped
+
lifecycle-aware
+
transactional
+
DB-only
+
safe against apparently active daemons
```

The important architectural boundary is:

```text
InspectClusterRemoval()
        ↓
operator safety policy
        ↓
RemoveCluster()
        ↓
database
        ↓
Kubernetes Web naturally reflects the cleaned state
```

For someone encountering the same problem again:

> Stop and permanently decommission the target cluster's Icinga Kubernetes daemon, inspect the cluster through the DB-only removal command, and only explicitly confirm removal when the lifecycle state is `stale`.

Do not manually delete only the `cluster` row, and do not treat a stale heartbeat by itself as proof that deletion is safe.