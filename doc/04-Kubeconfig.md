# Kubeconfig

Kubernetes uses a configuration file, typically referred to as **kubeconfig**,
to manage cluster access details and user credentials. It is created when you create a new cluster or
when you create a new user for an existing cluster.

## Purpose of the Kubeconfig

This file enables the `kubectl` command-line tool to interact with the
Kubernetes cluster, allowing users to perform various operations such as deploying applications, inspecting cluster
resources, and managing cluster configurations.

Like `kubectl`, Icinga for Kubernetes requires a kubeconfig file to access the Kubernetes API.

## Managing Kubeconfig

### Location of Kubeconfig File

By default, the kubeconfig file is located at `~/.kube/config` on most systems.


#### Changing the Location
You can specify a different location for the kubeconfig file by setting the KUBECONFIG environment variable.
This allows you to use a configuration file stored in another location.


```shell
export KUBECONFIG=/path/to/your/kubeconfig
````

### Structure of the Kubeconfig File

A typical kubeconfig file is a YAML file with the following main sections:

- **apiVersion:** Specifies the version of the Kubernetes API.
- **kind:** Defines the type of Kubernetes resource being described.
- **clusters:** Defines the clusters that kubectl can connect to.
- **users:** Contains the authentication information for different users.
- **contexts:** Represents a combination of a cluster, a user, and a namespace.
- **current-context:** Indicates the default context that kubectl uses for commands.


#### Example kubeconfig file

The following example of a kubeconfig file illustrates the values that must be replaced by user-specific values.

```yml
apiVersion: v1
kind: Config
clusters:
  - cluster:
      server: https://your-cluster-api-server:8443
      certificate-authority: /path/to/ca.crt
    name: your-cluster
users:
  - name: your-user
    user:
      client-certificate: /path/to/client.crt
      client-key: /path/to/client.key
contexts:
  - context:
      cluster: your-cluster
      user: your-user
      namespace: default
    name: your-context
current-context: your-context
```

For a more detailed explanation of the kubeconfig file, see the [official Kubernetes documentation](https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/).
