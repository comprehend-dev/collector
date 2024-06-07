package models

type Model interface {
	toJSON() ([]byte, error)
}
