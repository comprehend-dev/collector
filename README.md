The agent collects data from your local system for importing into comprehend. This includes
collectors for database schemas (PostgreSQL, MariaDB and MySQL) and Kubernetes clusters
(deployments, jobs, cronjobs, pods, nodes and warning events, plus resource usage metrics).

# Configuration

```
Usage: ./agent [options] [arguments]
Options:
    --config <path>         A configuration file containing additional options.
    --apikey <value>        The comprehend.dev API key you created ("API Keys" in the menu).
    --organization <value>  Your comprehend.dev organization slug.
    --postgresql <value>    PostgreSQL connection string/URI.
    --mariadb <value>       MariaDB connection string/URI (DSN).
    --mysql <value>         MySQL connection string/URI (DSN).
    --k8s <value>           Kubernetes namespace, or "<kubeconfig path> <namespace>".
Arguments:
    postgresql://...        PostgreSQL connection string/URI.
    mariadb://...           MariaDB connection string/URI.
    mysql://...             MySQL connection string/URI.
```

When run without any arguments the agent will try to use collector specific environment variables
(e.g. `PG_HOST`) and auto-detect things to report.

Each collector can be configured via a similarly named command line option (e.g. `--postgresql`)
taking collector specific configuration data or via a URI command line argument.

## Configuration file

Instead of command line arguments, the agent can also be configured via an ini-style configuration
file. Each section configures a single collector; non-unique sections are allowed, so a section can
be repeated to collect from several databases or namespaces.

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

If the agent is running inside a pod in the cluster it picks up the in-cluster configuration
automatically. Outside the cluster it uses your kubectl configuration (`~/.kube/config` by default).
Pass a namespace to limit collection to that namespace (e.g. `--k8s default`), or omit it to collect
from all namespaces. To point at a specific kubeconfig, give the path and namespace separated by a
space, e.g. `--k8s '~/.kube/config default'`.

Per-pod and per-node CPU/memory metrics require the
[metrics server](https://github.com/kubernetes-sigs/metrics-server). The agent tolerates missing
RBAC permissions: it logs a warning once for any resource it cannot read and continues with whatever
it can collect.
