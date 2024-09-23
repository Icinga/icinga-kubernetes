<!-- {% if index %} -->
# Installing Icinga for Kubernetes

![Icinga for Kubernetes](res/icinga-kubernetes-installation.png)

## Using Helm

For deploying Icinga for Kubernetes and its dependencies within a Kubernetes cluster,
the recommended approach is to use our
[Helm charts](https://github.com/Icinga/helm-charts/tree/main/charts/icinga-stack) to
deploy a ready-to-use Icinga stack.

## Alternative Installation Methods

Though any of the Icinga for Kubernetes components can run either inside or outside Kubernetes clusters,
including the database, common setup approaches include the following:

* All components run inside a Kubernetes cluster.
* All components run outside a Kubernetes cluster.
* Only the Icinga for Kubernetes daemon runs inside a Kubernetes cluster,
  requiring configuration for an external service to connect to the database outside the cluster.

### Setting up the Database

A MySQL (≥8.0) or MariaDB (≥10.5) database is required to run Icinga for Kubernetes.
Please follow the steps, which guide you through setting up the database and user, and importing the schema.

#### Setting up a MySQL or MariaDB Database

Set up a MySQL database for Icinga for Kubernetes:

```
CREATE DATABASE kubernetes;
CREATE USER 'kubernetes'@'localhost' IDENTIFIED BY 'CHANGEME';
GRANT ALL ON kubernetes.* TO 'kubernetes'@'localhost';
```

Icinga for Kubernetes automatically imports the schema on first start and also applies schema migrations if required.

### Running Within Kubernetes

Instead of using Helm charts, you can deploy Icinga for Kubernetes using the
[sample configuration](../icinga-kubernetes.example.yml).
First, create a local copy and adjust the database credentials as needed,
and modify the connection configuration if necessary.
The sample configuration provides an overview of general settings,
and all available settings are detailed under [Configuration](03-Configuration.md).

### Running Out-of-Cluster

#### Installing via Package

To install Icinga for Kubernetes outside of a Kubernetes cluster,
it is recommended to use prebuilt packages available for all supported platforms from
our official release [repository](https://packages.icinga.com).
Follow the steps provided for your target operating system to set up the repository and
install Icinga for Kubernetes via the `icinga-kubernetes` package.

##### Configuring Icinga for Kubernetes

Icinga for Kubernetes installs its configuration file to `/etc/icinga-kubernetes/config.yml`,
pre-populating most of the settings for a local setup. Before running Icinga for Kubernetes,
adjust the database credentials and, if necessary, the connection configuration.
The configuration file explains general settings.
All available settings can be found under [Configuration](03-Configuration.md).

##### Running Icinga for Kubernetes

The `icinga-kubernetes` package automatically installs the required systemd unit files to run Icinga for Kubernetes.
To connect to a Kubernetes cluster, a locally accessible
[kubeconfig](https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/) file is needed.
You can specify which kubeconfig file to use by setting the `KUBECONFIG` environment variable for
the Icinga for Kubernetes systemd service.
To do this, run `systemctl edit icinga-kubernetes` and add the following:

```bash
[Service]
Environment="KUBECONFIG=..."
```

Please run the following command to enable and start the Icinga for Kubernetes service:

```bash
systemctl enable --now icinga-kubernetes
```

#### Using a Container

Before running Icinga for Kubernetes, create a local `config.yml` using [the sample configuration](../config.example.yml)
adjust the database credentials and, if necessary, the connection configuration.
The configuration file explains general settings.
All available settings can be found under [Configuration](03-Configuration.md).

With locally accessible
[kubeconfig](https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/)
and `config.yml` files,
run the `icinga/icinga-kubernetes` image using a container runtime of you choice, e.g. Docker:

```bash
export KUBECONFIG=$HOME/.kube/config
export ICINGA_KUBERNETES_CONFIG=config.yml
docker run --rm -v $ICINGA_KUBERNETES_CONFIG:/config.yml -v $KUBECONFIG:/.kube/config icinga/icinga-kubernetes:edge
```

#### From Source

##### Using `go install`

You can build and install `icinga-kubernetes` as follows:

```bash
go install github.com/icinga/icinga-kubernetes@latest
```

This should place the `icinga-kubernetes` binary in your configured `$GOBIN` path which defaults to `$GOPATH/bin` or
`$HOME/go/bin` if the `GOPATH` environment variable is not set.

##### Build from Source

Download or clone the source and run the following command from the source's root directory.

```bash
go build -o icinga-kubernetes cmd/icinga-kubernetes/main.go
```

##### Configuring Icinga for Kubernetes

Before running Icinga for Kubernetes, create a local `config.yml` using [the sample configuration](../config.example.yml)
adjust the database credentials and, if necessary, the connection configuration.
The configuration file explains general settings.
All available settings can be found under [Configuration](03-Configuration.md).

##### Running Icinga for Kubernetes

With locally accessible
[kubeconfig](https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/)
and `config.yml` files, `icinga-kubernetes` can be executed by running:

```bash
icinga-kubernetes -config /path/to/config.yml
```

### Kubernetes Access Control Requirements

Icinga for Kubernetes requires the following read-only permissions on all resources within a Kubernetes cluster:

* **get**: Allows to retrieve details of resources.
* **list**: Allows to list all instances of resources.
* **watch**: Allows to watch for changes to resources.

You can grant these permissions by creating a `ClusterRole` with the necessary rules and
binding it to an appropriate service account or user.
Below is an example `ClusterRole` configuration:

```
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: icinga-for-kubernetes
rules:
  - apiGroups: [ "*" ]
    resources: [ "*" ]
    verbs: [ "get", "list", "watch" ]
```

A complete example of the Kubernetes RBAC configuration is included in the
[sample configuration](../icinga-kubernetes.example.yml). As a result,
you don't need to manually configure access when deploying Icinga for Kubernetes using the sample configuration or our
[Helm charts](https://github.com/Icinga/helm-charts/tree/main/charts/icinga-stack).

**When running Icinga for Kubernetes outside of a Kubernetes cluster,
it is required to connect as a user with the necessary permissions.**

### Installing Icinga for Kubernetes Web

With Icinga for Kubernetes and the database fully set up, you have completed the instructions here and can proceed to
[installing Icinga for Kubernetes Web](https://icinga.com/docs/icinga-kubernetes-web/latest/doc/02-Installation/)
which connects to the database to display and work with the monitoring data.
<!-- {% endif %} -->
