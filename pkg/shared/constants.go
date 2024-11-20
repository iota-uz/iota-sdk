package shared

import "github.com/go-playground/form"

var Decoder = form.NewDecoder()

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
