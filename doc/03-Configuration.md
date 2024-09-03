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

Icinga for Kubernetes can synchronize metrics from Prometheus using the Prometheus API.
The configuration for Prometheus is stored in the `prometheus` section of the [config.example.yml](../config.example.yml) file.

| Option | Description                                                       |
|--------|-------------------------------------------------------------------|
| url    | **Required.** The URL (`[Host]:[Port]`) of the Prometheus server. |

Ensure that the URL points to a running Prometheus instance that collects metrics from your Kubernetes cluster.