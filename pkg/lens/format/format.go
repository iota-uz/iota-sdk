// Package format defines Lens formatter specs and value formatting helpers.
package format

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

type Kind string

const (
	KindMoney            Kind = "money"
	KindAbbreviatedMoney Kind = "abbreviated_money"
	KindInteger          Kind = "integer"
	KindPercent          Kind = "percent"
	KindDate             Kind = "date"
	KindMonthLabel       Kind = "month_label"
	KindDuration         Kind = "duration"
	KindLocalizedString  Kind = "localized_string"
)

type Spec struct {
	Name       string
	Kind       Kind
	Currency   string
	Precision  int
	Layout     string
	Dictionary map[string]string
}

type Formatter interface {
	Format(value any, locale, timezone string) string
}

func MoneyCompact(currency string) Spec {
	return Spec{Name: "money_compact", Kind: KindAbbreviatedMoney, Currency: currency, Precision: 2}
}

func Count() Spec {
	return Spec{Name: "count", Kind: KindInteger}
}

func Percent(precision int) Spec {
	return Spec{Name: "percent", Kind: KindPercent, Precision: precision}
}

func Date(layout string) Spec {
	return Spec{Name: "date", Kind: KindDate, Layout: layout}
}

func Apply(spec *Spec, value any, locale, timezone string) string {
	if spec == nil {
		return defaultFormat(value)
	}
	switch spec.Kind {
	case KindMoney:
		number, ok := coerceNumber(value)
		if !ok {
			return defaultFormat(value)
		}
		return fmt.Sprintf("%.*f %s", spec.Precision, number, spec.Currency)
	case KindAbbreviatedMoney:
		number, ok := coerceNumber(value)
		if !ok {
			return defaultFormat(value)
		}
		return fmt.Sprintf("%s %s", abbreviate(number, spec.Precision), spec.Currency)
	case KindInteger:
		number, ok := coerceNumber(value)
		if !ok {
			return defaultFormat(value)
		}
		return fmt.Sprintf("%.0f", math.Round(number))
	case KindPercent:
		number, ok := coerceNumber(value)
		if !ok {
			return defaultFormat(value)
		}
		precision := spec.Precision
		if precision < 0 {
			precision = 0
		}
		return fmt.Sprintf("%.*f%%", precision, number)
	case KindDate:
		layout := spec.Layout
		if layout == "" {
			layout = "2006-01-02"
		}
		timestamp, ok := coerceTime(value, timezone)
		if !ok {
			return defaultFormat(value)
		}
		return timestamp.Format(layout)
	case KindMonthLabel:
		timestamp, ok := coerceTime(value, timezone)
		if !ok {
			return defaultFormat(value)
		}
		return timestamp.Format("Jan 2006")
	case KindDuration:
		duration, ok := coerceDuration(value)
		if !ok {
			return defaultFormat(value)
		}
		return duration.String()
	case KindLocalizedString:
		if text, ok := value.(string); ok {
			if localized, exists := spec.Dictionary[text]; exists {
				return localized
			}
			return text
		}
		return defaultFormat(value)
	default:
		return defaultFormat(value)
	}
}

func defaultFormat(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case time.Time:
		return v.Format("2006-01-02")
	case *time.Time:
		if v == nil {
			return ""
		}
		return v.Format("2006-01-02")
	case float64:
		return fmt.Sprintf("%.2f", v)
	case float32:
		return fmt.Sprintf("%.2f", v)
	default:
		return fmt.Sprint(v)
	}
}

func abbreviate(value float64, precision int) string {
	abs := math.Abs(value)
	switch {
	case abs >= 1_000_000_000:
		return fmt.Sprintf("%.*fB", precision, value/1_000_000_000)
	case abs >= 1_000_000:
		return fmt.Sprintf("%.*fM", precision, value/1_000_000)
	case abs >= 1_000:
		return fmt.Sprintf("%.*fK", precision, value/1_000)
	default:
		return fmt.Sprintf("%.*f", precision, value)
	}
}

func coerceNumber(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err == nil {
			return parsed, true
		}
		return 0, false
	default:
		return 0, false
	}
}

func coerceTime(value any, timezone string) (time.Time, bool) {
	switch v := value.(type) {
	case time.Time:
		return applyTimezone(v, timezone), true
	case *time.Time:
		if v == nil {
			return time.Time{}, false
		}
		return applyTimezone(*v, timezone), true
	case string:
		trimmed := strings.TrimSpace(v)
		if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
			return applyTimezone(parsed, timezone), true
		}
		for _, layout := range []string{"2006-01-02", "2006-01-02 15:04:05"} {
			parsed, err := parseTimeInLocation(layout, trimmed, timezone)
			if err == nil {
				return applyTimezone(parsed, timezone), true
			}
		}
		return time.Time{}, false
	default:
		return time.Time{}, false
	}
}

func coerceDuration(value any) (time.Duration, bool) {
	switch v := value.(type) {
	case time.Duration:
		return v, true
	case int8:
		return time.Duration(v) * time.Second, true
	case int16:
		return time.Duration(v) * time.Second, true
	case int:
		return time.Duration(v) * time.Second, true
	case int32:
		return time.Duration(v) * time.Second, true
	case int64:
		return time.Duration(v) * time.Second, true
	case uint:
		return time.Duration(v) * time.Second, true
	case uint8:
		return time.Duration(v) * time.Second, true
	case uint16:
		return time.Duration(v) * time.Second, true
	case uint32:
		return time.Duration(v) * time.Second, true
	case uint64:
		return time.Duration(v) * time.Second, true
	case uintptr:
		return time.Duration(v) * time.Second, true
	case float32:
		return time.Duration(float64(v) * float64(time.Second)), true
	case float64:
		return time.Duration(v * float64(time.Second)), true
	case string:
		trimmed := strings.TrimSpace(v)
		if duration, err := time.ParseDuration(trimmed); err == nil {
			return duration, true
		}
		if seconds, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return time.Duration(seconds * float64(time.Second)), true
		}
		return 0, false
	default:
		return 0, false
	}
}

func applyTimezone(value time.Time, timezone string) time.Time {
	if timezone == "" {
		return value
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return value
	}
	return value.In(location)
}

func parseTimeInLocation(layout, value, timezone string) (time.Time, error) {
	if timezone == "" {
		return time.Parse(layout, value)
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Parse(layout, value)
	}
	return time.ParseInLocation(layout, value, location)
}
