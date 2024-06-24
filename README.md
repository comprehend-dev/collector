The agent collects data from your local system for importing into comprehend. This includes
collectors for database schemas (currently only PostgreSQL), kubernetes clusters (not yet
implemented), containers (not yet implemented), etc.

# Configuration

```
Usage: ./agent [options] [arguments]
Options:
    --apikey <value>        The comprehend.dev API key you created ("API Keys" in the menu).
    --organization <value>  Your comprehend.dev organization slug.
    --document <value>      The document to import the schema to.
    --postgresql <value>    PostgreSQL connection string/URI
Arguments:
    postgresql://...        PostgreSQL connection string/URI
```

When run without any arguments the agent will try to use collector specific environment variables
(e.g. `PG_HOST`) and auto-detect things to report.

Each collector can be configured via a similarly named command line option (e.g. `--postgresql`)
taking collector specific configuration data or via a URI command line argument.

## Configuration file

Instead of command line arguments, the agent can also be configured via an ini-style configuration
file:

```
apikey=0123456789abcdef
organization=comprehend
document=arch

[postgresql]
host=localhost
dbname=comprehend
sslmode=disable
```

## PostgreSQL

Note that Go's Postgres driver defaults to localhost instead of a UNIX socket and that it enables
SSL mode by default. Pass the path to the directory containing the socket as host to connect via
UNIX socket, e.g. `--postgresql 'host=/run/postgresql'`. Add `?sslmode=disable` to the URI or
sslmode=disable to the connection string if SSL is not enabled on your server.
