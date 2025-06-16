package crud

import "time"

func WithKey(key bool) FieldOption {
	return func(field *field) {
		field.key = key
	}
}

func WithReadonly(readonly bool) FieldOption {
	return func(field *field) {
		field.readonly = readonly
	}
}

func WithHidden(hidden bool) FieldOption {
	return func(field *field) {
		field.hidden = hidden
	}
}

func WithSearchable(searchable bool) FieldOption {
	return func(field *field) {
		field.searchable = searchable
	}
}

func WithInitialValue(initialValue any) FieldOption {
	return func(field *field) {
		field.initialValue = initialValue
	}
}

func WithRules(rules []FieldRule) FieldOption {
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

func WithMultiline(multiline bool) FieldOption {
	return func(field *field) {
		field.attrs[Multiline] = multiline
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

func WithTrim(trim bool) FieldOption {
	return func(field *field) {
		field.attrs[Trim] = trim
	}
}

func WithUppercase(uppercase bool) FieldOption {
	return func(field *field) {
		field.attrs[Uppercase] = uppercase
	}
}

func WithLowercase(lowercase bool) FieldOption {
	return func(field *field) {
		field.attrs[Lowercase] = lowercase
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

func WithWeekdaysOnly(weekdaysOnly bool) FieldOption {
	return func(field *field) {
		field.attrs[WeekdaysOnly] = weekdaysOnly
		if weekdaysOnly {
			field.rules = append(field.rules, WeekdayRule())
		}
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
