package shared

import (
	"github.com/go-playground/form"
	"time"
)

var Decoder = initDecoder()
var Encoder = form.NewEncoder()

type DateOnly time.Time

func initDecoder() *form.Decoder {
	decoder := form.NewDecoder()
	decoder.RegisterCustomTypeFunc(func(strings []string) (interface{}, error) {
		v, err := time.Parse(time.DateOnly, strings[0])
		if err != nil {
			return nil, err
		}
		return DateOnly(v), nil
	}, DateOnly{})
	return decoder
}

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
