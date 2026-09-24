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

If Cluster B is permanently decommissioned, its daemon stops writing new data, but its synchronized database state
remains.

The result is:

```text
Cluster B permanently decommissioned
        ↓
its icinga-kubernetes daemon stops
        ↓
old Cluster B database state remains
        ↓
Cluster B remains available through Kubernetes Web
```

Deleting only the row from the `cluster` table is not sufficient. A monitored cluster owns resource rows, dependent
relationship rows, container state, metrics, configuration and daemon-instance state.

---

## Final Operator Behaviour

Cluster removal is an explicit DB-only administrative operation:

```bash
icinga-kubernetes \
    --remove-cluster <CLUSTER_UUID>
```

The command performs the removal immediately.

There is no separate lifecycle-inspection or confirmation phase in the final implementation.

The operator must therefore first permanently decommission the target cluster from this Icinga for Kubernetes
database and stop the daemon that synchronizes it.

The removal path executes before normal Kubernetes client initialization. A reachable Kubernetes API or working
kubeconfig is not required for the cleanup operation.

---

## Final Runtime Structure

The implementation is split across three responsibilities:

```text
cmd/icinga-kubernetes/main.go
        │
        │ parse UUID
        │ load normal configuration
        │ connect to existing database
        ▼
internal/remove_cluster.go
        │
        │ own complete cluster-removal transaction
        │ select target resource UUIDs
        │ orchestrate normal + special cleanup
        ▼
pkg/database/database.go
        │
        └── DeleteTx()
              │
              ├── existing Relations()
              ├── CascadeDelete()
              ├── BuildDeleteStmt()
              ├── batching
              └── caller-owned *sqlx.Tx
```

`internal/remove_cluster.go` is the orchestration layer because it can depend on both the database package and the
schema package without reversing their existing dependency direction.

---

## Reuse-First Deletion Design

The previous implementation contained a large manually maintained cluster deletion graph.

The final implementation instead reuses the repository's existing resource relationship metadata wherever that
metadata correctly expresses deletion ownership.

For ordinary resources the flow is:

```text
resource factory
        ↓
select UUIDs belonging to target cluster
        ↓
DeleteTx(..., WithCascading())
        ↓
existing resource Relations()
        ↓
delete cascading relation rows
        ↓
delete resource root rows
```

That means relationship knowledge continues to live primarily in the schema resource models instead of being copied
into a second general-purpose cluster cleanup registry.

---

## Why `YieldAll()` + `DeleteStreamed()` Are Not Used Directly

The repository already contains `YieldAll()` and `DeleteStreamed()`, and that existing machinery strongly influenced
the simplified design.

They were not copied directly into the final cluster-removal path because cluster removal has one additional
requirement: the complete destructive operation must remain inside one caller-owned SQL transaction.

### Transaction ownership

`YieldAll()` performs its query through the database object:

```text
YieldAll()
    ↓
db.query(...)
```

It does not accept a caller-owned `*sqlx.Tx`.

`DeleteStreamed()` similarly reaches the normal database bulk-execution path:

```text
DeleteStreamed()
    ↓
BulkExec()
    ↓
database executor
```

Using those two functions directly would therefore move selection and/or deletion work outside the transaction owned
by `RemoveCluster()`.

The cluster-removal implementation instead performs its UUID selections through:

```text
tx.SelectContext(...)
```

and its deletions through:

```text
DeleteTx(..., tx, ...)
```

so the complete mutation remains under the same transaction.

### Streaming mechanics

A literal synchronous producer such as:

```text
create unbuffered delete channel
        ↓
send UUID into channel
        ↓
start DeleteStreamed later
```

would block on the first send because no receiver has started yet.

Likewise, a deletion consumer placed after a producer loop that only exits on cancellation does not form a valid
producer/consumer pipeline.

The existing streaming helpers solve these problems through concurrent goroutines. Cluster removal avoids adding a
second concurrent streaming pipeline because its stronger requirement is transaction ownership, not streaming
throughput.

### Concrete resource types

Schema resource factories return concrete pointer-backed resources.

For example, a DaemonSet factory returns a `Resource` whose concrete value is `*DaemonSet`. `DaemonSet` embeds `Meta`,
but the interface's dynamic type is not `Meta` itself.

The removal implementation therefore does not depend on an assertion such as:

```text
entity.(Meta)
```

It selects only the database UUID into the small `clusterRemovalUuid` shape required for deletion.

### Cascading is explicit

`DeleteStreamed()` only traverses a resource's `Relations()` when cascading is enabled.

A generic root deletion without:

```text
WithCascading()
```

would remove only the resource root row and leave cascading relationship rows behind.

The final implementation preserves that same feature semantic through `DeleteTx()`.

---

## Why `DeleteTx()` Exists

`DeleteTx()` is deliberately a small extension of the existing deletion machinery rather than a second independent
deletion system.

It reuses:

```text
HasRelations
Relations()
CascadeDelete()
BuildDeleteStmt()
MaxPlaceholdersPerStatement
Feature handling
```

but executes the resulting deletes through the `*sqlx.Tx` supplied by the caller.

Conceptually:

```text
DeleteStreamed()
    existing relationship semantics
    existing delete-statement semantics
    database-owned execution

DeleteTx()
    existing relationship semantics
    existing delete-statement semantics
    caller-owned transaction execution
```

This preserves the repository's authoritative relationship metadata while allowing `RemoveCluster()` to retain
cluster-wide atomic rollback.

---

## Why Some Cleanup Is Still Explicit

Not every table needed by cluster removal is represented safely by the normal zero-value resource relation graph.

Those cases remain deliberately small and explicit.

### Persistent Volumes

`PersistentVolume.Relations()` returns no relations when the zero-value resource has no `Claim`.

A factory-created descriptor therefore cannot be blindly cascaded during administrative removal, because other
PersistentVolume relation rows may still exist.

Cluster removal supplies the required PersistentVolume relation descriptors explicitly while retaining shared global
label and annotation records.

### Pod Containers

`Pod.Relations()` deliberately marks:

```text
containers
init containers
sidecar containers
```

as `WithoutCascadeDelete()`.

Containers also own their own child data.

Cluster removal therefore removes target-cluster container metrics and container trees before deleting the Pod roots.

### Services

The current Service relation metadata reaches `ResourceAnnotations` through more than one foreign-key path.

The streaming deletion implementation groups cascading relation channels by relation table name, so blindly sending
that duplicate same-table relation shape through the generic streaming path would not safely represent both paths.

Cluster removal therefore uses a small explicit Service relation list with the required foreign keys.

This is kept local to the cluster-removal exception instead of broadening Issue #218 into an unrelated schema-model
refactor.

### Metrics and direct cluster state

Prometheus node, pod and container metrics are not all represented as normal cascading resource relations.

Cluster-level Prometheus metrics, configuration and `kubernetes_instance` rows are also directly scoped by
`cluster_uuid`.

Those rows are therefore cleaned explicitly before the final cluster row is removed.

---

## Transaction Boundary

`RemoveCluster()` owns one transaction around the complete operation:

```text
BEGIN
  ↓
container metrics
  ↓
container trees
  ↓
resource-specific metric / relation cleanup
  ↓
normal resources through existing Relations()
  ↓
PersistentVolume / Service special relations
  ↓
cluster metrics
  ↓
configuration
  ↓
kubernetes_instance
  ↓
cluster row LAST
  ↓
COMMIT
```

If any intermediate SQL operation fails:

```text
ROLLBACK
```

The cluster row is removed last so a successful transaction cannot leave a surviving cluster selector entry after
its cluster-owned state has been removed.

---

## Operator Safety Boundary

The final CLI does not perform heartbeat classification and does not decide whether a cluster is operationally safe
to remove.

The safety boundary is therefore explicit operator intent:

```text
permanently decommission target
        ↓
stop its icinga-kubernetes daemon
        ↓
verify target database
        ↓
verify target cluster UUID
        ↓
run --remove-cluster
```

If the stopped daemon is later restarted against the same database, it can synchronize that cluster again.

This is why stopping and permanently decommissioning the correct daemon remains an important prerequisite even
though it is no longer enforced by a runtime heartbeat policy.

---

## Why No Kubernetes Web Deletion Code Is Needed

Kubernetes Web derives its available cluster information from database state.

Therefore:

```text
RemoveCluster()
      ↓
Cluster B database state removed
      ↓
Cluster B cluster row removed
      ↓
next Web query
      ↓
Cluster B no longer appears as an available cluster
```

The Web module does not need to maintain a second deletion graph.

A browser session that was previously fixed to the removed cluster may need to select `All clusters` and refresh.

---

## Runtime Validation

The transaction-capable replacement was exercised against restored disposable two-cluster MariaDB fixtures before
the superseded implementation was removed.

Successful removal proved:

```text
Cluster B removal                PASS
Cluster A isolation              PASS
20 direct cluster-owned tables   PASS
112 relationship paths           PASS
3 container-metric paths         PASS
5 retained shared/global tables  PASS
141 deep checks                  PASS
Cluster B orphan rows            0
```

Repeat removal of the already-absent Cluster B was then run against the complete 98-table database. The command
returned success, Cluster A remained present, Cluster B remained absent, and every table retained exactly the same
row count.

A controlled failure was also introduced after earlier resource deletion stages had already begun. The real removal
reached the injected SQL fault and failed. The transaction restored the earlier deletes, all 98 table counts exactly
matched the frozen pre-failure database again, and both Cluster A and Cluster B remained present.

That rollback proof is the reason transaction ownership is not merely an architectural preference in this
implementation: it is a runtime-tested property of the removal operation.

---

## Final Resolution

Issue #218 is resolved through an operation that is:

```text
explicit
+
cluster-scoped
+
transactional
+
DB-only
+
reuse-oriented
+
idempotent for an already-absent target
```

The important implementation principle is:

> Reuse the existing schema relationship and deletion knowledge wherever it correctly represents the data, extend
> the generic deletion primitive only enough to preserve the required transaction boundary, and keep explicit
> cleanup limited to relationships that the existing generic metadata cannot safely express.

For operators:

> Permanently decommission the target, stop its Icinga for Kubernetes daemon, verify the database and cluster UUID,
> then run `icinga-kubernetes --remove-cluster <CLUSTER_UUID>`.

Do not manually delete only the `cluster` row.
