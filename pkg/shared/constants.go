package shared

import (
	"time"

	"github.com/go-playground/form"
)

var Decoder = initDecoder()
var Encoder = form.NewEncoder()

type DateOnly time.Time

func initDecoder() *form.Decoder {
	decoder := form.NewDecoder()
	decoder.RegisterCustomTypeFunc(func(strings []string) (interface{}, error) {
		if len(strings) == 0 {
			return DateOnly(time.Time{}), nil
		}
		if len(strings[0]) == 0 {
			return DateOnly(time.Time{}), nil
		}
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
