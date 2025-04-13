package filters

import (
	"fmt"
	"time"
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

// DateFormatter is a simple formatter for date values
func DateFormatter(value any) string {
	if ts, ok := value.(time.Time); ok {
		return fmt.Sprintf(`<div x-data="relativeformat"><span x-text="format('%s')">%s</span></div>`,
			ts.Format(time.RFC3339),
			ts.Format("2006-01-02 15:04:05"))
	}
	return fmt.Sprintf("%v", value)
}
