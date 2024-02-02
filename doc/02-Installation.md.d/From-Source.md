# Installing Icinga Kubernetes from Source

## Using `go install`

You can build and install `icinga-kubernetes` as follows:

```bash
go install github.com/icinga/icinga-kubernetes@latest
```

This should place the `icinga-kubernetes` binary in your configured `$GOBIN` path which defaults to `$GOPATH/bin` or
`$HOME/go/bin` if the `GOPATH` environment variable is not set.

## Build from Source

Download or clone the source and run the following command from the source's root directory.

```bash
go build -o icinga-kubernetes cmd/icinga-kubernetes/main.go
```

<!-- {% set from_source = True %} -->
<!-- {% include "02-Installation.md" %} -->

## Configuring Icinga Kubernetes

Before running Icinga Kubernetes, create a local `config.yml` using [the sample configuration](../../config.example.yml)
adjust the database credentials and, if necessary, the connection configuration.
The configuration file explains general settings.
All available settings can be found under [Configuration](../03-Configuration.md).

## Running Icinga Kubernetes

With locally accessible kubeconfig and `config.yml` files, `icinga-kubernetes` can be executed by running:

```bash
icinga-kubernetes -config /path/to/config.yml
```

## Using a Container

With locally accessible kubeconfig and `config.yml` files,
run the `icinga/icinga-kubernetes` image using a container runtime of you choice, e.g. Docker:

```bash
export KUBECONFIG=$HOME/.kube/config
export ICINGA_KUBERNETES_CONFIG=config.yml
docker run --rm --network=host -v $ICINGA_KUBERNETES_CONFIG:/config.yml -v $KUBECONFIG:/.kube/config icinga/icinga-kubernetes:edge
```
