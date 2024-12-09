package collectors

import (
	"database/sql"
	"github.com/comprehend-dev/comprehend.dev/agent/models"
	"github.com/go-ini/ini"
	"github.com/go-sql-driver/mysql"
	"strings"
)

type MySQLCollector struct {
	Collector
	connStr string
}

func (c MySQLCollector) Initialize(arg string) (Collector, error) {
	collector := MySQLCollector{nil, arg}
	db, err := sql.Open("mysql", collector.connStr)
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

func (c MySQLCollector) InitializeFromConfig(section *ini.Section) (Collector, error) {
	var sb strings.Builder
	for _, key := range section.Keys() {
		sb.WriteString(key.Name())
		sb.WriteString("=")
		sb.WriteString(key.String())
		sb.WriteString(" ")
	}
	return c.Initialize(sb.String())
}

func (c MySQLCollector) InitializeDefault() (Collector, error) {
	return c.Initialize("")
}

func (c MySQLCollector) URISchema() (string) {
	return "mysql"
}

func (c MySQLCollector) Description() (string) {
	return "MySQL connection string/URI"
}

func (c MySQLCollector) Collect() (models.Model, error) {
	config, err := mysql.ParseDSN(c.connStr)
	if err != nil {
		return nil, err
	}
	if (config.Params == nil) {
		config.Params = make(map[string]string)
	}
	/* In MySQL, the order of elements in arrays created by json_arrayagg is
	   undefined. There's no good way to get around that, so we have to avoid
	   this function. A workaround is to assemble the array as a string with
	   the help of group_concat, wrapping that in square brackets) and casting
	   the result back to JSON. However group_concat has a very low default
	   limit for the length of the resulting string (1024). Thus we need to
	   extend the limit in the connection settings. */
	config.Params["group_concat_max_len"] = "1048576"
	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
		select json_arrayagg(sch)
		from
		(
			select
				json_object(
					'database', catalog_name,
					'schema', schema_name,
					'tables', case when count(tables.tbl) = 0 then json_array() else json_arrayagg(tables.tbl) end
				) as sch
			from information_schema.schemata
				left join (
					select
						table_catalog,
						table_schema,
						json_object(
							'name', tables.table_name,
							'columns', columns.cols,
							'rows', coalesce(table_rows, -1),
							'primary_key', pk_cols.primary_key,
							'unique_keys', coalesce((
								select json_arrayagg(cons) as cons
								from (
									select
										table_schema,
										table_name,
										constraint_name,
										json_object('column_names', cast(concat('[', group_concat(json_quote(column_name) order by ordinal_position), ']') as json)) as cons
									from information_schema.key_column_usage
										natural join information_schema.table_constraints
									where constraint_type = 'UNIQUE'
									group by table_schema, table_name, constraint_name
								) uniques
								where uniques.table_schema = tables.table_schema and uniques.table_name = tables.table_name
							), json_array()),
							'foreign_keys', coalesce(fkeys.cons, json_array()),
							'indexes', coalesce((
								select json_arrayagg(idx) as idxs
								from (
									select
										table_schema,
										table_name,
										index_name,
										json_object(
											'column_names', cast(concat('[', group_concat(json_quote(column_name) order by seq_in_index), ']') as json)
										) as idx
									from information_schema.statistics statistics
									where index_name != 'PRIMARY'
									group by table_schema, table_name, index_name
								) idxs
								where idxs.table_schema = tables.table_schema and idxs.table_name = tables.table_name
							), json_array())
						) as tbl
					from information_schema.tables tables
						natural left join (
							select
								table_schema,
								table_name,
								cast(concat('[', group_concat(
										json_object(
											'name', column_name,
											'type', data_type,
											'nullable', is_nullable = 'YES'
										) order by ordinal_position
									), ']') as json
								) as cols
							from information_schema.columns
							group by table_schema, table_name
						) as columns
						natural left join (
							select table_schema, table_name, json_object('column_names', cast(concat('[', group_concat(json_quote(column_name) order by ordinal_position), ']') as json)) as primary_key
							from information_schema.key_column_usage
								natural join information_schema.table_constraints
							where constraint_type = 'PRIMARY KEY'
							group by table_schema, table_name
						) pk_cols
						natural left join (
							select table_schema, table_name, json_arrayagg(cons) as cons
							from (
								select
									table_schema,
									table_name,
									constraint_name,
									json_object(
										'referenced_table', referenced_table_name,
										'column_names', json_arrayagg(column_name),
										'referenced_columns', json_arrayagg(referenced_column_name)
									) as cons
								from information_schema.key_column_usage
									natural join information_schema.table_constraints
								where constraint_type = 'FOREIGN KEY'
								group by table_schema, table_name, referenced_table_name, constraint_name
							) fk_cols
							group by table_schema, table_name
						) fkeys
					where table_schema != 'information_schema'
					group by tables.table_catalog, tables.table_schema, tables.table_name
				) tables on tables.table_catalog = catalog_name and tables.table_schema = schema_name
			where schema_name != 'information_schema'
			group by catalog_name, schema_name
		) as  schs
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rows.Next();
	var databases string;
	if err := rows.Scan(&databases); err != nil {
		return nil, err
	}

	return *models.NewJSONModel(databases), nil
}

var registeredMySQL = RegisterCollector(MySQLCollector{})
