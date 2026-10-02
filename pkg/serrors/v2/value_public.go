package serrors

import (
	"context"

	"github.com/iota-uz/go-i18n/v2/i18n"
)

// PublicValue projects an explicitly declared localization value.
func PublicValue(value Value, l *i18n.Localizer) any {
	if value.message != nil {
		return Localize(*value.message, l, "")
	}
	return value.scalar
}

// FindPublicCarrier stops at an explicit classification so an outer internal
// error cannot disclose the public contract of a nested cause.
func FindPublicCarrier[T any](err error) (T, bool) {
	var zero T
	found := false
	walk(err, func(node error) bool {
		if carrier, ok := node.(T); ok {
			zero = carrier
			found = true
			return true
		}
		if semantic, ok := node.(*Error); ok && semantic.code != 0 {
			return true
		}
		return node == context.Canceled || node == context.DeadlineExceeded
	})
	return zero, found
}
