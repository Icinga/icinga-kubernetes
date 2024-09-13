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

## Prometheus Configuration

Connection configuration for a Prometheus instance that collects metrics from your Kubernetes cluster,
from which Icinga for Kubernetes [synchronizes predefined metrics](01-About.md#metric-sync) to display charts in the UI.
Defined in the `prometheus` section of the configuration file.

| Option | Description                                                                          |
|--------|--------------------------------------------------------------------------------------|
| url    | **Optional.** Prometheus server URL. If not set, metric synchronization is disabled. |
