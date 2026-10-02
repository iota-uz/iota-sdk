package serrors

import "github.com/iota-uz/go-i18n/v2/i18n"

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
	if err == nil {
		return zero, false
	}
	if carrier, ok := err.(T); ok {
		return carrier, true
	}
	if semantic, ok := err.(*Error); ok && semantic.code != 0 {
		return zero, false
	}
	switch wrapped := err.(type) {
	case interface{ Unwrap() []error }:
		for _, child := range wrapped.Unwrap() {
			if carrier, ok := FindPublicCarrier[T](child); ok {
				return carrier, true
			}
		}
	case interface{ Unwrap() error }:
		return FindPublicCarrier[T](wrapped.Unwrap())
	}
	return zero, false
}
