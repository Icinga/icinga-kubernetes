# Configuration via YAML File

The configuration is stored in `/etc/icinga-kubernetes/config.yml`.
See [config.example.yml](../config.example.yml) for an example configuration.

## Database Configuration

Connection configuration for the database to which Icinga for Kubernetes synchronizes monitoring data.
This is also the database used in
[Icinga for Kubernetes Web](https://icinga.com/docs/icinga-kubernetes-web) to view and work with the data.

| Option   | Description                                                       |
|----------|-------------------------------------------------------------------|
| type     | **Optional.** Only `mysql` is supported yet which is the default. |
| host     | **Required.** Database host or absolute Unix socket path.         |
| port     | **Optional.** Database port. By default, the MySQL port.          |
| database | **Required.** Database name.                                      |
| user     | **Required.** Database username.                                  |
| password | **Optional.** Database password.                                  |
| tls      | **Optional.** Whether to use TLS.                                 |
| cert     | **Optional.** Path to TLS client certificate.                     |
| key      | **Optional.** Path to TLS private key.                            |
| ca       | **Optional.** Path to TLS CA certificate.                         |
| insecure | **Optional.** Whether not to verify the peer.                     |

## Logging Configuration

| Env      | Description                                                                                                                                                              |
|----------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| level    | **Optional.** Default logging level. Can be set to `fatal`, `error`, `warn`, `info` or `debug`. If not set, defaults to `info`.                                          |
| output   | **Optional.** Logging output. Can be set to `console` (stderr) or `systemd-journald`. If not set, logs to systemd-journald when running under systemd, otherwise stderr. |
| interval | **Optional.** Interval for periodic logging defined as duration string. Valid units are `ms`, `s`, `m`, `h`. Defaults to `20s`.                                          |

## Notifications Configuration

Connection configuration for [Icinga Notifications](https://github.com/icinga/icinga-notifications) daemon.
If one of `url`, `username`, or `password` is set, **all** must be set.
Defined in the `notifications` section of the configuration file.

| Option             | Description                                                                                           |
|--------------------|-------------------------------------------------------------------------------------------------------|
| url                | **Optional.** Icinga Notifications daemon URL. If not set, notifications are disabled                 | 
| username           | **Optional.** Username for authenticating the Icinga for Kubernetes source in Icinga Notifications.   |
| password           | **Optional.** Password for authenticating the Icinga for Kubernetes source in Icinga Notifications.   |
| kubernetes_web_url | **Optional.** The base URL of Icinga for Kubernetes Web used in generated Icinga Notification events. |

## Prometheus Configuration

Connection configuration for a Prometheus instance that collects metrics from your Kubernetes cluster,
from which Icinga for Kubernetes [synchronizes predefined metrics](01-About.md#metric-sync) to display charts in the UI.
Defined in the `prometheus` section of the configuration file.

| Option   | Description                                                                                                                |
|----------|----------------------------------------------------------------------------------------------------------------------------|
| url      | **Optional.** Prometheus server URL. If not set, metric synchronization is disabled.                                       |
| insecure | **Optional.** Skip the TLS/SSL certificate verification. Can be set to 'true' or 'false'. If not set, defaults to 'false'. |
