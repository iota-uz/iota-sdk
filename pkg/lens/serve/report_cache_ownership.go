package serve

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/shopspring/decimal"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/chrome"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/resource"
)

func ownReportSpec(source lens.DashboardSpec, limit int64) (lens.DashboardSpec, int64, error) {
	_, err := resource.EstimateProjected(source, limit, reflect.TypeFor[chrome.Icon]())
	if err != nil {
		return lens.DashboardSpec{}, 0, err
	}
	cloned, err := cloneSpecValue(reflect.ValueOf(source))
	if err != nil {
		return lens.DashboardSpec{}, 0, err
	}
	owned := cloned.Interface().(lens.DashboardSpec)
	size, err := resource.Estimate(owned, limit)
	if err != nil {
		return lens.DashboardSpec{}, 0, err
	}
	return owned, size, nil
}

// OwnReportDatasets preflights and deeply owns producer datasets while
// preserving typed literals, dates and static frame indexes. Reserve the
// returned structural size before retaining these domain-owned datasets.
func OwnReportDatasets(source []lens.DatasetSpec, limit int64) ([]lens.DatasetSpec, int64, error) {
	owned, size, err := ownReportSpec(lens.DashboardSpec{Datasets: source}, limit)
	return owned.Datasets, size, err
}

func ownPreparedDocument(source *document.DashboardDocument, limit int64) (*document.DashboardDocument, error) {
	type wireDocument document.DashboardDocument
	if _, err := resource.Estimate((*wireDocument)(source), limit); err != nil {
		return nil, err
	}
	owned, err := cloneSpecValue(reflect.ValueOf(source))
	if err != nil {
		return nil, err
	}
	return owned.Interface().(*document.DashboardDocument), nil
}

func cloneSpecValue(value reflect.Value) (reflect.Value, error) {
	if !value.IsValid() {
		return value, nil
	}
	if value.Type() == reflect.TypeFor[time.Time]() {
		return value, nil
	}
	if value.Type() == reflect.TypeFor[chrome.Icon]() {
		// React wire documents do not render host templ components. Do not
		// retain their opaque callback captures inside a report registration.
		return reflect.Zero(value.Type()), nil
	}
	if value.Type() == reflect.TypeFor[frame.Frame]() {
		original := value.Interface().(frame.Frame)
		return reflect.ValueOf(*original.Clone()), nil
	}
	if value.Type() == reflect.TypeFor[decimal.Decimal]() {
		return reflect.ValueOf(value.Interface().(decimal.Decimal).Copy()), nil
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
	case reflect.Invalid, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64, reflect.String:
		return value, nil
	case reflect.Uintptr, reflect.Complex64, reflect.Complex128, reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return reflect.Value{}, fmt.Errorf("unsupported report value %s", value.Kind())
	}
	return reflect.Value{}, fmt.Errorf("unsupported report value %s", value.Kind())
}

type derivedBuildOwnershipKey struct{}

type derivedChildReceipt struct {
	forget func()
	touch  func(context.Context) error
}

type derivedBuildOwnership struct {
	mu            sync.Mutex
	children      map[string]*derivedChildReceipt
	closed        bool
	maxChildren   int
	acquireParent func(context.Context) (func(), error)
	sessions      *SessionRegistry
	dataScope     string
}

func (o *derivedBuildOwnership) add(id string, receipt *derivedChildReceipt) error {
	o.mu.Lock()
	if o.closed {
		o.mu.Unlock()
		receipt.forget()
		return context.Canceled
	}
	if o.children[id] == nil && len(o.children) >= o.maxChildren {
		requested := len(o.children) + 1
		o.mu.Unlock()
		receipt.forget()
		return resource.NewLimitError("derived_reports", int64(requested), int64(o.maxChildren))
	}
	previous := o.children[id]
	o.children[id] = receipt
	o.mu.Unlock()
	if previous != nil {
		previous.forget()
	}
	return nil
}

func (o *derivedBuildOwnership) take(id string) *derivedChildReceipt {
	o.mu.Lock()
	o.closed = true
	selected := o.children[id]
	delete(o.children, id)
	remaining := o.children
	o.children = nil
	o.mu.Unlock()
	for _, child := range remaining {
		child.forget()
	}
	return selected
}
