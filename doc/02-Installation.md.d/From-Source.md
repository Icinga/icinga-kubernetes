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
