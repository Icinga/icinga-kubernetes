# Commands

## Removing a Cluster

Icinga for Kubernetes keeps the data of every monitored cluster in the database
until it is removed explicitly. Stopping or uninstalling the daemon of a cluster
leaves its data in place, and the cluster still appears in
[Icinga for Kubernetes Web](https://icinga.com/docs/icinga-kubernetes-web).

To remove a cluster you no longer monitor, run `icinga-kubernetes` with the
`--remove-cluster` flag. It deletes the cluster and all data that belongs to it
from the database. The command only connects to the database, so it does not
need a kubeconfig and works after the cluster itself is gone.

!!! Warning

    The removal cannot be undone.

### Stopping the Instance

Stop the Icinga for Kubernetes instance that monitors the cluster before
removing it. A running instance keeps writing the cluster's data to the
database.

* For a package installation, stop and remove the service instance as described
  in [Configuring multiple Instances](02-Installation.md#configuring-multiple-instances-of-icinga-for-kubernetes-for-multi-cluster-support).
* For a Helm installation, uninstall the release.

### Finding the Cluster UUID

Query the `cluster` table to list the UUIDs and names of all clusters in the
database:

```sql
SELECT HEX(uuid) as uuid, name FROM cluster;
```

If the cluster is still reachable, its UUID is also the UID of its
`kube-system` namespace:

```bash
kubectl get namespace kube-system -o jsonpath='{.metadata.uid}'
```

### Running the Removal

Run `icinga-kubernetes` with the same database configuration the instance used
and pass the cluster UUID to `--remove-cluster`:

```bash
icinga-kubernetes --config /etc/icinga-kubernetes/config.yml --remove-cluster <uuid>
```

If the instance's environment file overrides database settings, set the same
`ICINGA_FOR_KUBERNETES_DATABASE_*` variables for this command. All database
options are listed under
[Configuration](03-Configuration.md#configuration-via-environment-variables).

On success, the command logs `Removed cluster "<uuid>"` and exits with status 0.
It fails with an error if the UUID is invalid or the database connection fails.
If no cluster with that UUID exists, it logs a warning and still removes any
remaining data of that cluster, so you can rerun an interrupted removal.
