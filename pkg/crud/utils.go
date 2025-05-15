package crud

import (
	"encoding"
	"fmt"
	"reflect"
	"strconv"
)

// parseID converts the incoming URL string into the generic ID type.
func parseID[ID any](idStr string) (ID, error) {
	var id ID
	// first, handle the common scalar cases via a type switch on the zero-value
	switch any(id).(type) {
	case string:
		return any(idStr).(ID), nil
	case int:
		v, err := strconv.Atoi(idStr)
		return any(v).(ID), err
	case int64:
		v, err := strconv.ParseInt(idStr, 10, 64)
		return any(v).(ID), err
	case uint, uint64:
		v, err := strconv.ParseUint(idStr, 10, 64)
		// strconv.ParseUint returns a uint64; convert down if needed
		if err != nil {
			return id, err
		}
		// perform safe narrowing if ID is uint
		switch any(id).(type) {
		case uint:
			return any(uint(v)).(ID), nil
		default:
			return any(v).(ID), nil
		}
	}

	// next, see if *ID implements TextUnmarshaler (e.g. for UUID types)
	ptr := reflect.New(reflect.TypeOf(id))
	if tu, ok := ptr.Interface().(encoding.TextUnmarshaler); ok {
		if err := tu.UnmarshalText([]byte(idStr)); err != nil {
			return id, fmt.Errorf("invalid %T: %w", id, err)
		}
		// ptr is *ID; we need the value
		return ptr.Elem().Interface().(ID), nil
	}

	return id, fmt.Errorf("unsupported ID type %T", id)
}
