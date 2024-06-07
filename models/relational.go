package models

import (
	"encoding/json"
)

type RelationalModel struct {
	Tables []Table `json:"tables"`
}

func (m RelationalModel) toJSON() ([]byte, error) {
	return json.Marshal(m)
}

type Table struct {
	Name        string       `json:"name"`
	Columns     []Column     `json:"columns"`
	Rows        int64        `json:"rows"`
	PrimaryKey  PrimaryKey   `json:"primary_key"`
	UniqueKeys  []UniqueKey  `json:"unique_keys"`
	ForeignKeys []ForeignKey `json:"foreign_keys"`
	Indexes     []Index      `json:"indexes"`
}

type Column struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}

type PrimaryKey struct {
	ColumnNames []string `json:"column_names"`
}

type UniqueKey struct {
	ColumnNames []string `json:"column_names"`
}

type ForeignKey struct {
	ColumnNames       []string `json:"column_names"`
	ReferencedTable   string   `json:"referenced_table"`
	ReferencedColumns []string `json:"referenced_columns"`
}

type Index struct {
	ColumnNames []string `json:"column_names"`
}
