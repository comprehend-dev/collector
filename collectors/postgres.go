package collectors

import (
	"database/sql"
	"github.com/comprehend-dev/collector/models"
	_ "github.com/lib/pq"
	"github.com/go-ini/ini"
	"net/url"
	"strconv"
	"strings"
)

type PostgresCollector struct {
	Collector
	connStr  string
	hostInfo *HostInfo
}

func parsePostgresHostInfo(connStr string) *HostInfo {
	host := "localhost"
	port := 5432

	// Try URI format first: postgresql://user:pass@host:port/db
	if strings.HasPrefix(connStr, "postgres://") || strings.HasPrefix(connStr, "postgresql://") {
		if u, err := url.Parse(connStr); err == nil {
			if u.Hostname() != "" {
				host = u.Hostname()
			}
			if p, err := strconv.Atoi(u.Port()); err == nil {
				port = p
			}
			return &HostInfo{Host: host, Port: port}
		}
	}

	// Key=value format: host=localhost port=5432 ...
	for _, part := range strings.Fields(connStr) {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		switch k {
		case "host":
			host = v
		case "port":
			if p, err := strconv.Atoi(v); err == nil {
				port = p
			}
		}
	}
	return &HostInfo{Host: host, Port: port}
}

func (c PostgresCollector) Initialize(arg string) (Collector, error) {
	collector := PostgresCollector{nil, arg, parsePostgresHostInfo(arg)}
	db, err := sql.Open("postgres", collector.connStr)
	if err != nil {
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		return nil, err
	}
	db.Close()
	return collector, nil
}

func (c PostgresCollector) HostInfo() *HostInfo {
	return c.hostInfo
}

func (c PostgresCollector) InitializeFromConfig(section *ini.Section) (Collector, error) {
	var sb strings.Builder
	for _, key := range section.Keys() {
		sb.WriteString(key.Name())
		sb.WriteString("=")
		sb.WriteString(key.String())
		sb.WriteString(" ")
	}
	return c.Initialize(sb.String())
}

func (c PostgresCollector) InitializeDefault() (Collector, error) {
	return c.Initialize("")
}

func (c PostgresCollector) URISchema() (string) {
	return "postgresql"
}

func (c PostgresCollector) Description() (string) {
	return "PostgreSQL connection string/URI"
}

func (c PostgresCollector) Collect() (models.Model, error) {
	db, err := sql.Open("postgres", c.connStr)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
		with indexes as (
			select indexrelid, indrelid, indisprimary, indisunique, json_build_object('column_names', json_agg(attname)) as idx
			from (select indexrelid, indrelid, indisprimary, indisunique, unnest(indkey) as key from pg_index) as indkeys
				join pg_attribute on attrelid = indrelid and attnum = indkeys.key
			group by indexrelid, indrelid, indisprimary, indisunique
		),
		constraints as (
			select conkeys.oid, conrelid, contype, json_build_object('column_names', json_agg(c.attname), 'referenced_table', relname, 'referenced_columns', json_agg(r.attname)) as con
			from (select oid, conrelid, contype, confrelid, unnest(conkey) as key, unnest(confkey) as rkey from pg_constraint) as conkeys
				join pg_attribute c on c.attrelid = conkeys.conrelid and c.attnum = conkeys.key
				join pg_attribute r on r.attrelid = conkeys.confrelid and r.attnum = conkeys.rkey
				left join pg_class on confrelid = pg_class.oid
			group by conkeys.oid, conrelid, contype, relname
		)
		select json_agg(json_build_object('database', current_database(), 'schema', nspname, 'tables', coalesce(tables, '[]'::json)))
			from pg_namespace
				left join (
					select relnamespace,
					json_agg(json_build_object(
                                            'name', relname,
                                            'columns', cols,
					    'rows', case
						    when reltuples < 100000000 then (
							    xpath('/row/cnt/text()', query_to_xml(format('select count(*) as cnt from %I.%I', nspname, relname), false, true, ''))
						    )[1]::text::int
						    else reltuples end,
                                            'primary_key', primary_key.idx,
                                            'unique_keys', coalesce(uniqs, '[]'::json),
                                            'foreign_keys', coalesce(cons, '[]'::json),
                                            'indexes', coalesce(indxs, '[]'::json)
                                        )) as tables
					from pg_class
						join pg_namespace on pg_namespace.oid = pg_class.relnamespace
						left join (
							select attrelid, json_agg(json_build_object('name', attname, 'type', typname, 'nullable', not attnotnull) order by attnum) as cols
							from pg_attribute
								join pg_type on atttypid = pg_type.oid
							where attnum >= 1 and not attisdropped
							group by attrelid
						) as columns
						on columns.attrelid = pg_class.oid
						left join indexes as primary_key on primary_key.indrelid = pg_class.oid and primary_key.indisprimary
						left join(
							select indrelid, json_agg(idx) as uniqs
							from indexes
							where not indisprimary and indisunique
							group by indrelid
						) as uniqindexes
						on uniqindexes.indrelid = pg_class.oid
						left join(
							select indrelid, json_agg(idx) as indxs
							from indexes
							where not indisunique
							group by indrelid
						) as relindexes
						on relindexes.indrelid = pg_class.oid
						left join(
							select conrelid, json_agg(con) as cons
							from constraints
							where contype = 'f'
							group by conrelid
						) as fkconstraints
						on fkconstraints.conrelid = pg_class.oid
					where relkind in ('r', 'v', 'm', 'f', 'p')
						and nspname not like 'pg_%' and nspname != 'information_schema'
					group by relnamespace
				) as nsptables on nsptables.relnamespace = pg_namespace.oid
			where nspname not like 'pg_%' and nspname != 'information_schema'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rows.Next();
	var databases []byte;
	if err := rows.Scan(&databases); err != nil {
		return nil, err
	}

	return models.NewDatabaseModel(c.hostInfo.Host, c.hostInfo.Port, databases), nil
}

var registeredPG = RegisterCollector(PostgresCollector{})
