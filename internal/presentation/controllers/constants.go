package controllers

import "github.com/go-playground/form"

var decoder = form.NewDecoder()

type FormAction string

const (
	FormActionSave   FormAction = "save"
	FormActionDelete FormAction = "delete"
)

func (f *FormAction) IsValid() bool {
	switch *f {
	case FormActionSave, FormActionDelete:
		return true
	}
	return false
}
