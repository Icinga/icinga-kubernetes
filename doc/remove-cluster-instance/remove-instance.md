# Remove a Decommissioned Kubernetes Cluster

Use this procedure when a Kubernetes cluster has been **permanently decommissioned**, but its old resources and
cluster entry are still visible in Icinga for Kubernetes.

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

The removal command deletes the stored Icinga for Kubernetes database state belonging to the selected cluster.

> Only use this for a cluster that is intentionally and permanently being removed from this Icinga for Kubernetes
> database.

---

## Before You Start

Stop the Icinga for Kubernetes daemon that synchronizes the cluster you want to remove.

Do **not** restart that daemon after removing the cluster unless you intentionally want it to synchronize the cluster
into the database again.

You also need the UUID of the cluster you want to remove.

Example:

```text
12937015-6AF0-471D-8C47-8A581D38DE6A
```

!!! Warning

    `--remove-cluster` performs the database removal immediately.

    The command does not decide whether the target daemon is still active and does not require a separate confirmation
    option. Verify that the daemon has been stopped and that the UUID identifies the cluster you intend to remove
    before running the command.

---

## Remove the Cluster

Using the same database configuration as Icinga for Kubernetes, run:

```bash
icinga-kubernetes \
    --remove-cluster <CLUSTER_UUID>
```

For example:

```bash
icinga-kubernetes \
    --remove-cluster 12937015-6AF0-471D-8C47-8A581D38DE6A
```

A successful removal is reported as:

```text
Removed cluster <CLUSTER_UUID>
```

---

## What the Command Removes

The cleanup removes database state belonging to the selected cluster, including:

```text
cluster resources
resource relationship rows
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
remove instance / config / metric data
  ↓
remove cluster row last
  ↓
COMMIT
```

If a deletion step fails, the transaction is rolled back instead of intentionally leaving a partially removed cluster.

---

## What It Does Not Do

The command does not delete the Kubernetes cluster itself.

It removes that cluster's **stored Icinga for Kubernetes database state**.

The removal path does not require the old Kubernetes API server or kubeconfig to be reachable. It uses the configured
Icinga for Kubernetes database directly.

The command also does not stop a running Icinga for Kubernetes daemon for you. Stopping and permanently
decommissioning the correct daemon is an operational prerequisite.

---

## If You Use a Custom Config File

Use the normal Icinga for Kubernetes configuration option together with the removal command:

```bash
icinga-kubernetes \
    --config /path/to/config.yml \
    --remove-cluster <CLUSTER_UUID>
```

The selected configuration must point to the database from which the cluster should be removed.

---

## After Successful Removal

Because Kubernetes Web obtains its cluster information from the database, the removed cluster should no longer appear
as an available cluster after its stored state has been removed.

Refresh Kubernetes Web after the command completes.

If your browser session was previously fixed to the deleted cluster, select:

```text
All clusters
```

and refresh the page.

Also verify that the remaining monitored clusters still appear and continue to update normally.

---

## Quick Reference

```text
1. Permanently decommission the Kubernetes cluster.

2. Stop its Icinga for Kubernetes daemon.

3. Verify the database configuration that will be used.

4. Find and verify the target cluster UUID.

5. Run:

   icinga-kubernetes \
       --remove-cluster <CLUSTER_UUID>

6. Verify that the command reports successful removal.

7. Refresh Kubernetes Web.

8. Confirm that the removed cluster is gone.

9. Confirm that the other monitored clusters still work.
```

## Important

Do not manually solve this by deleting only:

```sql
DELETE FROM cluster ...
```

A monitored cluster owns additional resources and relationship rows.

Use the cluster-removal command so the cluster-scoped database cleanup is performed through the complete transactional
removal workflow.
