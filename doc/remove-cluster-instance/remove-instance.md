# Remove a Decommissioned Kubernetes Cluster

Use this procedure when a Kubernetes cluster has been **permanently decommissioned**, but its old resources and cluster entry are still visible in Icinga for Kubernetes.

## When to Use This

A typical situation looks like this:

```text
Cluster was monitored by Icinga for Kubernetes
        ↓
cluster is permanently decommissioned
        ↓
Icinga Kubernetes daemon stops
        ↓
old database data remains
        ↓
cluster still appears in Kubernetes Web
```

The removal command cleans the stored database state belonging to that cluster.

> Only use this for a cluster that is intentionally and permanently being removed from this Icinga for Kubernetes database.

---

## Before You Start

Make sure the Icinga for Kubernetes daemon for the cluster you want to remove has been stopped.

Do **not** restart that daemon after removing the cluster unless you intentionally want it to synchronize the cluster into the database again.

You also need the UUID of the cluster you want to remove.

Example:

```text
12937015-6AF0-471D-8C47-8A581D38DE6A
```

---

## Step 1 — Inspect the Cluster

Run the command first **without confirmation**:

```bash
icinga-kubernetes \
    --remove-cluster <CLUSTER_UUID>
```

For example:

```bash
icinga-kubernetes \
    --remove-cluster 12937015-6AF0-471D-8C47-8A581D38DE6A
```

This first command does not remove a stale cluster.

It checks the database and classifies the cluster as one of:

```text
missing
no_instance
active
stale
```

### `active`

The daemon has a recent heartbeat.

Removal is refused:

```text
active
    → DO NOT REMOVE
```

Stop the correct daemon and investigate why it is still reporting before continuing.

### `no_instance`

The cluster exists, but there is no daemon heartbeat available to make a reliable lifecycle decision.

Removal is refused:

```text
no_instance
    → DO NOT REMOVE
```

Investigate the cluster before deleting anything.

### `missing`

The cluster is already absent:

```text
missing
    → nothing to remove
```

No further action is required.

### `stale`

The cluster has an old daemon heartbeat:

```text
stale
    → eligible for explicit removal
```

If you have confirmed that this is the correct permanently decommissioned cluster, continue to Step 2.

---

## Step 2 — Remove the Stale Cluster

Run the same command with explicit confirmation:

```bash
icinga-kubernetes \
    --remove-cluster <CLUSTER_UUID> \
    --confirm-cluster-removal
```

Example:

```bash
icinga-kubernetes \
    --remove-cluster 12937015-6AF0-471D-8C47-8A581D38DE6A \
    --confirm-cluster-removal
```

The command will only perform the removal when the cluster is classified as `stale`.

The confirmation option does **not** override protection for an `active` cluster.

---

## What the Command Removes

The cleanup removes database state belonging to the selected cluster, including its:

```text
cluster resources
resource relationships
container-related state
Prometheus resource metrics
cluster configuration
kubernetes_instance records
cluster row
```

The operation is transactional:

```text
BEGIN
  ↓
remove dependent data
  ↓
remove cluster resources
  ↓
remove instance/config/metric data
  ↓
remove cluster row last
  ↓
COMMIT
```

If a deletion step fails, the transaction is rolled back instead of intentionally leaving the cluster partially removed.

---

## What It Does Not Do

The command does not delete the Kubernetes cluster itself.

It removes that cluster's **stored Icinga for Kubernetes database state**.

It also does not require the old Kubernetes API server or kubeconfig to still work.

The removal path uses the existing Icinga for Kubernetes database configuration directly.

---

## After Successful Removal

The command checks the cluster again.

The expected final state is:

```text
missing
```

Because Kubernetes Web obtains its cluster list from the database, the removed cluster should no longer appear as an available cluster after the database state is removed.

You may need to refresh Kubernetes Web.

If your browser session was previously fixed to the deleted cluster, select:

```text
All clusters
```

and refresh the page.

---

## Active Heartbeat Safety Window

The removal command uses a built-in `5m` active-heartbeat safety window.

If the newest recorded daemon heartbeat is no more than five minutes old, the cluster is classified as `active` and removal is refused.

This remains true even when `--confirm-cluster-removal` is supplied.

The five-minute window only determines whether the recorded heartbeat is considered `active` or `stale`. It does not authorize deletion. A `stale` cluster still requires explicit confirmation.

---

## If You Use a Custom Config File

Use the normal Icinga for Kubernetes configuration option together with the removal command:

```bash
icinga-kubernetes \
    --config /path/to/config.yml \
    --remove-cluster <CLUSTER_UUID>
```

Then, after confirming the cluster is `stale`:

```bash
icinga-kubernetes \
    --config /path/to/config.yml \
    --remove-cluster <CLUSTER_UUID> \
    --confirm-cluster-removal
```

---

## Quick Reference

```text
1. Permanently decommission the cluster.

2. Stop its Icinga for Kubernetes daemon.

3. Find the cluster UUID.

4. Inspect:

   icinga-kubernetes \
       --remove-cluster <CLUSTER_UUID>

5. Continue only if the result is stale.

6. Confirm removal:

   icinga-kubernetes \
       --remove-cluster <CLUSTER_UUID> \
       --confirm-cluster-removal

7. Verify the command reports successful removal.

8. Refresh Kubernetes Web.

9. Confirm the removed cluster is gone and other clusters still work.
```

## Important

Do not manually solve this by deleting only:

```sql
DELETE FROM cluster ...
```

A monitored cluster owns many additional resources and relationship rows.

Use the cluster-removal command so the complete cluster-owned database state is handled as one transactional lifecycle operation.
