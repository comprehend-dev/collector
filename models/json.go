package models

type JSONModel struct {
	data string
}

func (m JSONModel) toJSON() ([]byte, error) {
	return []byte(m.data), nil
}

func NewJSONModel(data string) *JSONModel {
    return &JSONModel{data}
}
