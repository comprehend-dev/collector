package collectors

import (
	"database/sql"
	"github.com/comprehend-dev/comprehend.dev/agent/models"
	"github.com/go-ini/ini"
	_ "github.com/go-sql-driver/mysql"
	"strings"
)

type MariaDBCollector struct {
	Collector
	connStr string
}

func (c MariaDBCollector) Initialize(arg string) (Collector, error) {
	collector := MariaDBCollector{nil, arg}
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

func (c MariaDBCollector) InitializeFromConfig(keys []*ini.Key) (Collector, error) {
	var sb strings.Builder
	for _, key := range keys {
		sb.WriteString(key.Name())
		sb.WriteString("=")
		sb.WriteString(key.String())
		sb.WriteString(" ")
	}
	return c.Initialize(sb.String())
}

func (c MariaDBCollector) InitializeDefault() (Collector, error) {
	return c.Initialize("")
}

func (c MariaDBCollector) URISchema() (string) {
	return "mariadb"
}

func (c MariaDBCollector) Description() (string) {
	return "MariaDB connection string/URI"
}

func (c MariaDBCollector) Collect() (models.Model, error) {
	db, err := sql.Open("mysql", c.connStr)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
		select json_arrayagg((
			select
				json_object(
					'database', catalog_name,
					'schema', schema_name,
					'tables', case when tables.tbl is null then json_array() else json_arrayagg(tables.tbl) end
				)
			from information_schema.schemata
				left join (
					select
						table_catalog,
						table_schema,
						json_object(
							'name', tables.table_name,
							'columns', columns.cols,
							'rows', table_rows,
							'primary_key', pk_cols.primary_key,
							'unique_keys', coalesce((
								select json_arrayagg(cons) as cons
								from (
									select
										table_schema,
										table_name,
										constraint_name,
										json_object('column_names', json_arrayagg(column_name order by ordinal_position)) as cons
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
											'column_names', json_arrayagg(column_name order by seq_in_index)
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
								json_arrayagg(
									json_object(
										'name', column_name,
										'type', data_type,
										'nullable', is_nullable = 'YES'
									) order by ordinal_position
								) as cols
							from information_schema.columns
							group by table_schema, table_name
						) as columns
						natural left join (
							select table_schema, table_name, json_object('column_names', json_arrayagg(column_name order by ordinal_position)) as primary_key
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
		))
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

var registeredMariaDB = RegisterCollector(MariaDBCollector{})
