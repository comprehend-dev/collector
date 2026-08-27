The comprehend.dev collector gathers data from your local system and ingests it into comprehend.dev.
It reads database schemas (PostgreSQL, MariaDB and MySQL) and Kubernetes clusters (deployments,
jobs, cronjobs, pods, nodes and warning events, plus resource usage metrics).

You will need a comprehend.dev API key and your organization slug to send anything. Both come from
the SDKs page in comprehend.dev: open your organization and choose SDKs from the menu, where you can
issue API keys and find setup instructions. That page belongs to your organization, so there is no
single address to link to from here.

# Installation

Download the archive for your platform from the
[releases page](https://github.com/comprehend-dev/collector/releases), check it against the
`SHA256SUMS` published alongside it, and unpack it:

```
tar -xzf collector-1.0.0-linux-amd64.tar.gz
./collector-1.0.0-linux-amd64/collector --version
```

Binaries are built for Linux and macOS, on both amd64 and arm64.

The container image is published for the same architectures; Docker picks the right one for your
machine:

```
docker run --rm ghcr.io/comprehend-dev/collector:latest --version
```

To build from source instead, you need Go 1.23 or later; `make` leaves a `collector` binary in the
working directory.

# Configuration

```
Usage: ./collector [options] [arguments]
Options:
    --config <path>         A configuration file containing additional options.
    --apikey <value>        The comprehend.dev API key you created ("API Keys" on the SDKs page).
    --organization <value>  Your comprehend.dev organization slug.
    --version               Print the collector version and exit.
    --k8s <value>           Kubernetes connection string/URI
    --mariadb <value>       MariaDB connection string/URI
    --mysql <value>         MySQL connection string/URI
    --postgresql <value>    PostgreSQL connection string/URI
Arguments:
    k8s://...               Kubernetes connection string/URI
    mariadb://...           MariaDB connection string/URI
    mysql://...             MySQL connection string/URI
    postgresql://...        PostgreSQL connection string/URI
```

When run without any arguments the collector will try to use data source specific environment
variables (e.g. `PG_HOST`) and auto-detect things to report.

Each data source can be configured via a similarly named command line option (e.g. `--postgresql`)
taking source specific configuration data or via a URI command line argument.

## Configuration file

Instead of command line arguments, the collector can also be configured via an ini-style
configuration file. Each section configures a single data source; non-unique sections are allowed,
so a section can be repeated to collect from several databases or namespaces.

```
apikey=0123456789abcdef
organization=comprehend

[postgresql]
host=localhost
dbname=comprehend
sslmode=disable

[k8s]
namespace=default
```

## PostgreSQL

Note that Go's Postgres driver defaults to localhost instead of a UNIX socket and that it enables
SSL mode by default. Pass the path to the directory containing the socket as host to connect via
UNIX socket, e.g. `--postgresql 'host=/run/postgresql'`. Add `?sslmode=disable` to the URI or
sslmode=disable to the connection string if SSL is not enabled on your server.

## Kubernetes

If the collector is running inside a pod in the cluster it picks up the in-cluster configuration
automatically. Outside the cluster it uses your kubectl configuration (`~/.kube/config` by default).
Pass a namespace to limit collection to that namespace (e.g. `--k8s default`), or omit it to collect
from all namespaces. To point at a specific kubeconfig, give the path and namespace separated by a
space, e.g. `--k8s '~/.kube/config default'`.

Per-pod and per-node CPU/memory metrics require the
[metrics server](https://github.com/kubernetes-sigs/metrics-server). The collector tolerates missing
RBAC permissions: it logs a warning once for any resource it cannot read and continues with whatever
it can collect.

# License

The collector is source available under the [Business Source License 1.1](LICENSE): you may read,
build, modify and redistribute it, and you may run it in production under the Additional Use Grant,
which covers everything except feeding collected data to a product or service competing with
comprehend.dev. Four years after a version is published, that version becomes available under the
Apache License, Version 2.0.

The collector links third-party Go packages, whose licenses are reproduced in
[THIRD-PARTY-LICENSES](THIRD-PARTY-LICENSES). Run `make third-party-licenses` to regenerate that
file after changing dependencies.
