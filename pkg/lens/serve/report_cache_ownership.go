package serve

import (
	"fmt"
	"reflect"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/chrome"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/resource"
)

func ownReportSpec(source lens.DashboardSpec, limit int64) (lens.DashboardSpec, int64, error) {
	size, err := resource.Estimate(source, limit)
	if err != nil {
		return lens.DashboardSpec{}, 0, err
	}
	cloned, err := cloneSpecValue(reflect.ValueOf(source))
	if err != nil {
		return lens.DashboardSpec{}, 0, err
	}
	return cloned.Interface().(lens.DashboardSpec), size, nil
}

func cloneSpecValue(value reflect.Value) (reflect.Value, error) {
	if !value.IsValid() {
		return value, nil
	}
	if value.Type() == reflect.TypeFor[time.Time]() {
		return value, nil
	}
	if value.Type() == reflect.TypeFor[chrome.Icon]() {
		if value.Interface().(chrome.Icon).Empty() {
			return value, nil
		}
		return reflect.Value{}, fmt.Errorf("cached reports cannot retain renderer icon callbacks")
	}
	if value.Type() == reflect.TypeFor[frame.Frame]() {
		original := value.Interface().(frame.Frame)
		return reflect.ValueOf(*original.Clone()), nil
	}
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type()), nil
		}
		child, err := cloneSpecValue(value.Elem())
		if err != nil {
			return reflect.Value{}, err
		}
		if value.Kind() == reflect.Pointer {
			out := reflect.New(value.Type().Elem())
			out.Elem().Set(child)
			return out, nil
		}
		out := reflect.New(value.Type()).Elem()
		out.Set(child)
		return out, nil
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type()), nil
		}
		out := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := 0; i < value.Len(); i++ {
			child, err := cloneSpecValue(value.Index(i))
			if err != nil {
				return reflect.Value{}, err
			}
			out.Index(i).Set(child)
		}
		return out, nil
	case reflect.Array:
		out := reflect.New(value.Type()).Elem()
		for i := 0; i < value.Len(); i++ {
			child, err := cloneSpecValue(value.Index(i))
			if err != nil {
				return reflect.Value{}, err
			}
			out.Index(i).Set(child)
		}
		return out, nil
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type()), nil
		}
		out := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			child, err := cloneSpecValue(iter.Value())
			if err != nil {
				return reflect.Value{}, err
			}
			out.SetMapIndex(iter.Key(), child)
		}
		return out, nil
	case reflect.Struct:
		out := reflect.New(value.Type()).Elem()
		for i := 0; i < value.NumField(); i++ {
			if value.Type().Field(i).PkgPath != "" {
				if value.Field(i).IsZero() {
					continue
				}
				return reflect.Value{}, fmt.Errorf("unsupported private report state %s", value.Type())
			}
			child, err := cloneSpecValue(value.Field(i))
			if err != nil {
				return reflect.Value{}, err
			}
			out.Field(i).Set(child)
		}
		return out, nil
	default:
		return value, nil
	}
}
