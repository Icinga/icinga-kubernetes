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
Defined in the `prometheus` section of the configuration file. If one of username or password is set, both must be set.

| Option   | Description                                                                                                                |
|----------|----------------------------------------------------------------------------------------------------------------------------|
| url      | **Optional.** Prometheus server URL. If not set, metric synchronization is disabled.                                       |
| insecure | **Optional.** Skip the TLS/SSL certificate verification. Can be set to 'true' or 'false'. If not set, defaults to 'false'. |
| username | **Optional.** Prometheus username.                                                                                         |
| password | **Optional.** Prometheus password.                                                                                         |

# Configuration via Environment Variables

**All** environment variables are prefixed with `ICINGA_FOR_KUBERNETES_`.
The database type would therefore be `ICINGA_FOR_KUBERNETES_DATABASE_TYPE`.
The configurations set by environment variables override the ones set by YAML.

## Database Configuration

| Env               | Description                                                       |
|-------------------|-------------------------------------------------------------------|
| DATABASE_TYPE     | **Optional.** Only `mysql` is supported yet which is the default. |
| DATABASE_HOST     | **Required.** Database host or absolute Unix socket path.         |
| DATABASE_PORT     | **Optional.** Database port. By default, the MySQL port.          |
| DATABASE_DATABASE | **Required.** Database name.                                      |
| DATABASE_USER     | **Required.** Database username.                                  |
| DATABASE_PASSWORD | **Optional.** Database password.                                  |

## Logging Configuration

| Env              | Description                                                                                                                                                              |
|------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| LOGGING_LEVEL    | **Optional.** Default logging level. Can be set to `fatal`, `error`, `warn`, `info` or `debug`. If not set, defaults to `info`.                                          |
| LOGGING_OUTPUT   | **Optional.** Logging output. Can be set to `console` (stderr) or `systemd-journald`. If not set, logs to systemd-journald when running under systemd, otherwise stderr. |
| LOGGING_INTERVAL | **Optional.** Interval for periodic logging defined as duration string. Valid units are `ms`, `s`, `m`, `h`. Defaults to `20s`.                                          |

## Notifications Configuration

| Env                              | Description                                                                                           |
|----------------------------------|-------------------------------------------------------------------------------------------------------|
| NOTIFICATIONS_URL                | **Optional.** Icinga Notifications daemon URL. If not set, notifications are disabled                 |
| NOTIFICATIONS_USERNAME           | **Optional.** Username for authenticating the Icinga for Kubernetes source in Icinga Notifications.   |
| NOTIFICATIONS_PASSWORD           | **Optional.** Password for authenticating the Icinga for Kubernetes source in Icinga Notifications.   |
| NOTIFICATIONS_KUBERNETES_WEB_URL | **Optional.** The base URL of Icinga for Kubernetes Web used in generated Icinga Notification events. |

## Prometheus Configuration

| Env                 | Description                                                                                                                |
|---------------------|----------------------------------------------------------------------------------------------------------------------------|
| PROMETHEUS_URL      | **Optional.** Prometheus server URL. If not set, metric synchronization is disabled.                                       |
| PROMETHEUS_INSECURE | **Optional.** Skip the TLS/SSL certificate verification. Can be set to 'true' or 'false'. If not set, defaults to 'false'. |
| PROMETHEUS_USERNAME | **Optional.** Prometheus username.                                                                                         |
| PROMETHEUS_PASSWORD | **Optional.** Prometheus password.                                                                                         |
