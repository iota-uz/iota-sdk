package details

import "reflect"

type PayoutOption func(d *payoutDetails)

func PayoutWithData(data map[string]any) PayoutOption {
	return func(d *payoutDetails) {
		d.data = clonePayoutData(data)
	}
}

func NewPayoutDetails(opts ...PayoutOption) PayoutDetails {
	d := &payoutDetails{data: make(map[string]any)}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

type payoutDetails struct {
	data map[string]any
}

func (d *payoutDetails) Data() map[string]any {
	return clonePayoutData(d.data)
}

func (d *payoutDetails) SetData(data map[string]any) PayoutDetails {
	result := *d
	result.data = clonePayoutData(data)
	return &result
}

func (d *payoutDetails) Get(key string) any {
	if d.data == nil {
		return nil
	}
	return clonePayoutValue(d.data[key])
}

func (d *payoutDetails) Set(key string, value any) PayoutDetails {
	result := *d
	result.data = clonePayoutData(d.data)
	if result.data == nil {
		result.data = make(map[string]any)
	}
	result.data[key] = clonePayoutValue(value)
	return &result
}

func clonePayoutData(data map[string]any) map[string]any {
	if data == nil {
		return nil
	}
	return clonePayoutReflect(
		reflect.ValueOf(data),
		make(map[payoutCloneVisit]reflect.Value),
	).Interface().(map[string]any)
}

func clonePayoutValue(value any) any {
	if value == nil {
		return nil
	}
	return clonePayoutReflect(
		reflect.ValueOf(value),
		make(map[payoutCloneVisit]reflect.Value),
	).Interface()
}

type payoutCloneVisit struct {
	kind      reflect.Kind
	valueType reflect.Type
	pointer   uintptr
	length    int
	capacity  int
}

func clonePayoutReflect(value reflect.Value, visited map[payoutCloneVisit]reflect.Value) reflect.Value {
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := clonePayoutReflect(value.Elem(), visited)
		result := reflect.New(value.Type()).Elem()
		result.Set(cloned)
		return result
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		visit := payoutCloneVisit{
			kind:      value.Kind(),
			valueType: value.Type(),
			pointer:   value.Pointer(),
			length:    0,
			capacity:  0,
		}
		if result, ok := visited[visit]; ok {
			return result
		}
		result := reflect.MakeMapWithSize(value.Type(), value.Len())
		visited[visit] = result
		iter := value.MapRange()
		for iter.Next() {
			result.SetMapIndex(iter.Key(), clonePayoutReflect(iter.Value(), visited))
		}
		return result
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		visit := payoutCloneVisit{
			kind:      value.Kind(),
			valueType: value.Type(),
			pointer:   value.Pointer(),
			length:    value.Len(),
			capacity:  value.Cap(),
		}
		if result, ok := visited[visit]; ok {
			return result
		}
		result := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		visited[visit] = result
		for i := range value.Len() {
			result.Index(i).Set(clonePayoutReflect(value.Index(i), visited))
		}
		return result
	case reflect.Array:
		result := reflect.New(value.Type()).Elem()
		for i := range value.Len() {
			result.Index(i).Set(clonePayoutReflect(value.Index(i), visited))
		}
		return result
	case reflect.Struct:
		result := reflect.New(value.Type()).Elem()
		result.Set(value)
		for i := range value.NumField() {
			if result.Field(i).CanSet() && value.Field(i).CanInterface() {
				result.Field(i).Set(clonePayoutReflect(value.Field(i), visited))
			}
		}
		return result
	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		visit := payoutCloneVisit{
			kind:      value.Kind(),
			valueType: value.Type(),
			pointer:   value.Pointer(),
			length:    0,
			capacity:  0,
		}
		if result, ok := visited[visit]; ok {
			return result
		}
		result := reflect.New(value.Type().Elem())
		visited[visit] = result
		result.Elem().Set(clonePayoutReflect(value.Elem(), visited))
		return result
	case reflect.Invalid,
		reflect.Bool,
		reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64,
		reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Uintptr,
		reflect.Float32,
		reflect.Float64,
		reflect.Complex64,
		reflect.Complex128,
		reflect.Chan,
		reflect.Func,
		reflect.String,
		reflect.UnsafePointer:
		return value
	}
	return value
}
