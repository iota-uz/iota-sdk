package product

import "errors"

const (
	InStock       Status = "in_stock"
	InDevelopment Status = "in_development"
	Approved      Status = "approved"
)

type Status string

func NewStatus(l string) (Status, error) {
	language := Status(l)
	if !language.IsValid() {
		return "", errors.New("invalid status")
	}
	return language, nil
}

func (l Status) IsValid() bool {
	switch l {
	case InStock, InDevelopment, Approved:
		return true
	}
	return false
}
