package clienthost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
)

const maxSafeInteger = int64(1<<53 - 1)

// validateSafeIntegers prevents Go integer values from silently losing
// precision when encoding/json hands them to JavaScript as JSON numbers.
func validateSafeIntegers(value any) error {
	if err := walkSafeIntegers(reflect.ValueOf(value), "$", make(map[visit]struct{})); err != nil {
		return err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var wire any
	if err := decoder.Decode(&wire); err != nil {
		return err
	}
	return walkJSONNumbers(wire, "$")
}

func walkJSONNumbers(value any, path string) error {
	switch typed := value.(type) {
	case json.Number:
		number, err := strconv.ParseFloat(typed.String(), 64)
		if err != nil || math.IsInf(number, 0) || math.IsNaN(number) {
			return fmt.Errorf("clienthost props %s: JSON number %q cannot be represented in JavaScript", path, typed.String())
		}
		if math.Abs(number) > float64(maxSafeInteger) {
			return fmt.Errorf("clienthost props %s: JSON number %q exceeds JavaScript safe range", path, typed.String())
		}
	case []any:
		for index, item := range typed {
			if err := walkJSONNumbers(item, fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
	case map[string]any:
		for key, item := range typed {
			if err := walkJSONNumbers(item, path+"."+key); err != nil {
				return err
			}
		}
	}
	return nil
}

type visit struct {
	typ reflect.Type
	ptr uintptr
}

func walkSafeIntegers(value reflect.Value, path string, seen map[visit]struct{}) error {
	if !value.IsValid() {
		return nil
	}
	if value.Kind() == reflect.Interface {
		if value.IsNil() {
			return nil
		}
		return walkSafeIntegers(value.Elem(), path, seen)
	}
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		key := visit{typ: value.Type(), ptr: value.Pointer()}
		if _, ok := seen[key]; ok {
			return nil
		}
		seen[key] = struct{}{}
		return walkSafeIntegers(value.Elem(), path, seen)
	}
	if value.CanInterface() {
		if _, ok := value.Interface().(json.Marshaler); ok {
			return nil
		}
	}
	switch value.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		integer := value.Int()
		if integer < -maxSafeInteger || integer > maxSafeInteger {
			return fmt.Errorf("clienthost props %s: integer %d exceeds JavaScript safe range", path, integer)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		integer := value.Uint()
		if integer > uint64(maxSafeInteger) {
			return fmt.Errorf("clienthost props %s: integer %d exceeds JavaScript safe range", path, integer)
		}
	case reflect.Float32, reflect.Float64:
		if math.IsInf(value.Float(), 0) || math.IsNaN(value.Float()) {
			return fmt.Errorf("clienthost props %s: non-finite number cannot be represented in JSON", path)
		}
	case reflect.Struct:
		typ := value.Type()
		for index := 0; index < value.NumField(); index++ {
			field := typ.Field(index)
			if !field.IsExported() || field.Tag.Get("json") == "-" {
				continue
			}
			name := field.Name
			if tag := field.Tag.Get("json"); tag != "" && tag[0] != ',' {
				for i, char := range tag {
					if char == ',' {
						name = tag[:i]
						break
					}
					name = tag
				}
				if name == "" {
					name = field.Name
				}
			}
			if err := walkSafeIntegers(value.Field(index), path+"."+name, seen); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		for index := 0; index < value.Len(); index++ {
			if err := walkSafeIntegers(value.Index(index), fmt.Sprintf("%s[%d]", path, index), seen); err != nil {
				return err
			}
		}
	case reflect.Map:
		iterator := value.MapRange()
		for iterator.Next() {
			if err := walkSafeIntegers(iterator.Value(), fmt.Sprintf("%s[%v]", path, iterator.Key()), seen); err != nil {
				return err
			}
		}
	}
	return nil
}
