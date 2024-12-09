package serrors

import (
	"encoding/json"
	"reflect"
)

func Unmarshal(body []byte, errInstance map[string]interface{}) error {
	var base Base
	if err := json.Unmarshal(body, &base); err != nil {
		return err
	}

	var instance interface{}
	if _, ok := errInstance[base.Code]; ok {
		instance = errInstance[base.Code]
	}

	target := reflect.New(reflect.TypeOf(instance))
	if err := json.Unmarshal(body, target.Interface()); err != nil {
		return err
	}
	return target.Elem().Interface().(error)
}
