package models

import "encoding/json"

type DatabaseModel struct {
	Host      string          `json:"host"`
	Port      int             `json:"port"`
	Databases json.RawMessage `json:"databases"`
}

func (m DatabaseModel) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

func NewDatabaseModel(host string, port int, databases []byte) *DatabaseModel {
	return &DatabaseModel{Host: host, Port: port, Databases: databases}
}
