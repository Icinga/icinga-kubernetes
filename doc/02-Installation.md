<!-- {% if index %} -->

# Installing Icinga Kubernetes

The recommended way to install Icinga Kubernetes is to use prebuilt packages for
all supported platforms from our official release repository.
Please follow the steps listed for your target operating system,
which guide you through setting up the repository and installing Icinga Kubernetes.

![Icinga Kubernetes](res/icinga-kubernetes-installation.png)

<!-- {% else %} -->
<!-- {% if not icingaDocs %} -->

## Installing the Package

If the [repository](https://packages.icinga.com) is not configured yet, please add it first.
Then use your distribution's package manager to install the `icinga-kubernetes` package
or install [from source](02-Installation.md.d/From-Source.md).
<!-- {% endif %} -->

## Setting up the Database

A MySQL (≥5.5) or MariaDB (≥10.1) database is required to run Icinga Kubernetes.
Please follow the steps listed for your target database,
which guide you through setting up the database and user and importing the schema.

### Setting up a MySQL or MariaDB Database

If you use a version of MySQL < 5.7 or MariaDB < 10.2, the following server options must be set:

```
innodb_file_format=barracuda
innodb_file_per_table=1
innodb_large_prefix=1
```

Set up a MySQL database for Icinga Kubernetes:

```
CREATE DATABASE kubernetes;
CREATE USER 'kubernetes'@'localhost' IDENTIFIED BY 'CHANGEME';
GRANT ALL ON kubernetes.* TO 'kubernetes'@'localhost';
```

After creating the database, import the Icinga Kubernetes schema located at
`/usr/share/kubernetes/schema/mysql/schema.sql`.

<!-- {% if not from_source %} -->
## Configuring Icinga Kubernetes

Icinga Kubernetes installs its configuration file to `/etc/icinga-kubernetes/config.yml`,
pre-populating most of the settings for a local setup. Before running Icinga Kubernetes,
adjust the database credentials and, if necessary, the connection configuration.
The configuration file explains general settings.
All available settings can be found under [Configuration](03-Configuration.md).

## Running Icinga Kubernetes

The `icinga-kubernetes` package automatically installs the necessary systemd unit files to run Icinga Kubernetes.
Please run the following command to enable and start its service:

```bash
systemctl enable --now icinga-kubernetes
```
<!-- {% endif %} -->

## Installing Icinga Kubernetes Web

With Icinga Kubernetes and the database fully set up, you have completed the instructions here and can proceed to
[installing Icinga Kubernetes Web](https://icinga.com/docs/icinga-kubernetes-web/latest/doc/02-Installation/)
which connects to the database to display and work with the monitoring data.
<!-- {% endif %} --><!-- {# end else if index #} -->