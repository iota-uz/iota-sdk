package shared

import "github.com/go-playground/form"

var Decoder = form.NewDecoder()
var Encoder = form.NewEncoder()

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
