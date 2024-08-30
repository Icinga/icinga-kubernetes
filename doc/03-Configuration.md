# Configuration

The configuration is stored in `/etc/icinga-kubernetes/config.yml`.
See [config.example.yml](../config.example.yml) for an example configuration.

## Database Configuration

Connection configuration for the database to which Icinga for Kubernetes synchronizes monitoring data.
This is also the database used in
[Icinga for Kubernetes Web](https://icinga.com/docs/icinga-kubernetes-web) to view and work with the data.

| Option   | Description                                                        |
|----------|--------------------------------------------------------------------|
| type     | **Optional.** Only `mysql` is supported yet which is the default.  |
| host     | **Required.** Database host or absolute Unix socket path.          |
| port     | **Optional.** Database port. By default, the MySQL port.           |
| database | **Required.** Database name.                                       |
| user     | **Required.** Database username.                                   |
| password | **Optional.** Database password.                                   |
| tls      | **Optional.** Whether to use TLS.                                  |
| cert     | **Optional.** Path to TLS client certificate.                      |
| key      | **Optional.** Path to TLS private key.                             |
| ca       | **Optional.** Path to TLS CA certificate.                          |
| insecure | **Optional.** Whether not to verify the peer.                      |

## Kubernetes Access Control Requirements
In order to guarantee that the Icinga Kubernetes monitoring system is able to function correctly within your Kubernetes
cluster, it is essential to configure RBAC (Role-Based Access Control) in an appropriate manner.

### Required Permissions
The monitoring system requires read-only access to all Kubernetes resources in the cluster. This is necessary to gather
data about the state of various resources, such as pods, nodes, and services. The required permissions include the 
following verbs and resources:

- Verbs: **get**, **watch**, **list**
- Resources: All resources across all API groups

### Example RBAC Configuration
An example of the requisite RBAC role and binding configuration for the Icinga Kubernetes monitoring system is 
provided in the [icinga-kubernetes.example.yml](../icinga-kubernetes.example.yml).
