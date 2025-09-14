package filters

import (
	"context"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

type Option func(t *TableFilter)

func WithPlaceholder(placeholder string) Option {
	return func(t *TableFilter) {
		t.placeholder = placeholder
	}
}

func MultiSelect() Option {
	return func(t *TableFilter) {
		t.multiple = true
	}
}

func WithMultiSelectClasses(classes templ.CSSClasses) Option {
	return func(t *TableFilter) {
		t.multiSelectClasses = classes
	}
}

func WithOptions(options ...OptionItem) Option {
	return func(t *TableFilter) {
		t.options = options
	}
}

func Opt(value, label string) OptionItem {
	return OptionItem{
		Value: value,
		Label: label,
	}
}

type OptionItem struct {
	Value string
	Label string
}

type TableFilter struct {
	Name               string
	placeholder        string
	multiSelectClasses templ.CSSClasses
	options            []OptionItem
	multiple           bool
}

func NewFilter(name string, opts ...Option) *TableFilter {
	f := &TableFilter{
		Name: name,
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

func (t *TableFilter) Add(opts ...OptionItem) *TableFilter {
	t.options = append(t.options, opts...)
	return t
}

func isOptionChecked(ctx context.Context, name string, opt OptionItem) bool {
	pgCtx := composables.UsePageCtx(ctx)
	query := pgCtx.URL.Query()
	if v := query.Get(name); v == "" {
		return false
	}
	for _, val := range query[name] {
		if val == opt.Value {
			return true
		}
	}
	return false
}
