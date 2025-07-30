package crud

import (
	"context"
	"time"
)

func WithKey() FieldOption {
	return func(field *field) {
		field.key = true
	}
}

func WithReadonly() FieldOption {
	return func(field *field) {
		field.readonly = true
	}
}

func WithHidden() FieldOption {
	return func(field *field) {
		field.hidden = true
	}
}

func WithSearchable() FieldOption {
	return func(field *field) {
		field.searchable = true
	}
}

func WithSortable() FieldOption {
	return func(field *field) {
		field.sortable = true
	}
}

func WithInitialValue(fn func(ctx context.Context) any) FieldOption {
	return func(field *field) {
		field.initialValueFn = fn
	}
}

func WithRules(rules ...FieldRule) FieldOption {
	return func(field *field) {
		field.rules = append(field.rules, rules...)
	}
}

func WithRule(rule FieldRule) FieldOption {
	return func(field *field) {
		field.rules = append(field.rules, rule)
	}
}

func WithAttrs(attrs map[string]any) FieldOption {
	return func(field *field) {
		for k, v := range attrs {
			field.attrs[k] = v
		}
	}
}

func WithAttr(key string, value any) FieldOption {
	return func(field *field) {
		field.attrs[key] = value
	}
}

func WithMinLen(minLen int) FieldOption {
	return func(field *field) {
		field.attrs[MinLen] = minLen
		field.rules = append(field.rules, MinLengthRule(minLen))
	}
}

func WithMaxLen(maxLen int) FieldOption {
	return func(field *field) {
		field.attrs[MaxLen] = maxLen
		field.rules = append(field.rules, MaxLengthRule(maxLen))
	}
}

func WithMultiline() FieldOption {
	return func(field *field) {
		field.attrs[Multiline] = true
	}
}

func WithMin(minValue int64) FieldOption {
	return func(field *field) {
		field.attrs[Min] = minValue
	}
}

func WithMax(maxValue int64) FieldOption {
	return func(field *field) {
		field.attrs[Max] = maxValue
	}
}

func WithFloatMin(minValue float64) FieldOption {
	return func(field *field) {
		field.attrs[Min] = minValue
	}
}

func WithFloatMax(maxValue float64) FieldOption {
	return func(field *field) {
		field.attrs[Max] = maxValue
	}
}

func WithPrecision(precision int) FieldOption {
	return func(field *field) {
		field.attrs[Precision] = precision
	}
}

func WithScale(scale int) FieldOption {
	return func(field *field) {
		field.attrs[Scale] = scale
	}
}

func WithDecimalMin(minValue string) FieldOption {
	return func(field *field) {
		field.attrs[Min] = minValue
	}
}

func WithDecimalMax(maxValue string) FieldOption {
	return func(field *field) {
		field.attrs[Max] = maxValue
	}
}

func WithMinDate(minDate time.Time) FieldOption {
	return func(field *field) {
		field.attrs[MinDate] = minDate
	}
}

func WithMaxDate(maxDate time.Time) FieldOption {
	return func(field *field) {
		field.attrs[MaxDate] = maxDate
	}
}

func WithPattern(pattern string) FieldOption {
	return func(field *field) {
		field.attrs[Pattern] = pattern
		field.rules = append(field.rules, PatternRule(pattern))
	}
}

func WithTrim() FieldOption {
	return func(field *field) {
		field.attrs[Trim] = true
	}
}

func WithUppercase() FieldOption {
	return func(field *field) {
		field.attrs[Uppercase] = true
	}
}

func WithLowercase() FieldOption {
	return func(field *field) {
		field.attrs[Lowercase] = true
	}
}

func WithStep(step int64) FieldOption {
	return func(field *field) {
		field.attrs[Step] = step
	}
}

func WithMultipleOf(multiple int64) FieldOption {
	return func(field *field) {
		field.attrs[MultipleOf] = multiple
		field.rules = append(field.rules, MultipleOfRule(multiple))
	}
}

func WithFloatStep(step float64) FieldOption {
	return func(field *field) {
		field.attrs[Step] = step
	}
}

func WithFormat(format string) FieldOption {
	return func(field *field) {
		field.attrs[Format] = format
	}
}

func WithTimezone(timezone string) FieldOption {
	return func(field *field) {
		field.attrs[Timezone] = timezone
	}
}

func WithWeekdaysOnly() FieldOption {
	return func(field *field) {
		field.attrs[WeekdaysOnly] = true
		field.rules = append(field.rules, WeekdayRule())
	}
}

func WithUUIDVersion(version int) FieldOption {
	return func(field *field) {
		field.attrs[UUIDVersion] = version
		field.rules = append(field.rules, UUIDVersionRule(version))
	}
}

func WithDefaultValue(defaultValue any) FieldOption {
	return func(field *field) {
		field.attrs[DefaultValue] = defaultValue
	}
}

func WithTrueLabel(label string) FieldOption {
	return func(field *field) {
		field.attrs[TrueLabel] = label
	}
}

func WithFalseLabel(label string) FieldOption {
	return func(field *field) {
		field.attrs[FalseLabel] = label
	}
}

func WithURL() FieldOption {
	return func(field *field) {
		field.rules = append(field.rules, URLRule())
	}
}

func WithPhone() FieldOption {
	return func(field *field) {
		field.rules = append(field.rules, PhoneRule())
	}
}

func WithAlpha() FieldOption {
	return func(field *field) {
		field.rules = append(field.rules, AlphaRule())
	}
}

func WithAlphanumeric() FieldOption {
	return func(field *field) {
		field.rules = append(field.rules, AlphanumericRule())
	}
}

func WithEmail() FieldOption {
	return func(field *field) {
		field.rules = append(field.rules, EmailRule())
	}
}

func WithRequired() FieldOption {
	return func(field *field) {
		field.rules = append(field.rules, RequiredRule())
	}
}

func WithPositive() FieldOption {
	return func(field *field) {
		field.rules = append(field.rules, PositiveRule())
	}
}

func WithNonNegative() FieldOption {
	return func(field *field) {
		field.rules = append(field.rules, NonNegativeRule())
	}
}

func WithNotEmpty() FieldOption {
	return func(field *field) {
		field.rules = append(field.rules, NotEmptyRule())
	}
}

func WithFutureDate() FieldOption {
	return func(field *field) {
		field.rules = append(field.rules, FutureDateRule())
	}
}

func WithPastDate() FieldOption {
	return func(field *field) {
		field.rules = append(field.rules, PastDateRule())
	}
}

func WithLocalizationKey(key string) FieldOption {
	return func(field *field) {
		field.localizationKey = key
	}
}
