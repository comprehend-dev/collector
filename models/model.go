package models

type Model interface {
	ToJSON() ([]byte, error)
}
