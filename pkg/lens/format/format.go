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

type Plugin interface {
	Name() string
	Build(spec Spec) (Formatter, error)
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
		number := toFloat(value)
		return fmt.Sprintf("%.*f %s", spec.Precision, number, spec.Currency)
	case KindAbbreviatedMoney:
		number := toFloat(value)
		return fmt.Sprintf("%s %s", abbreviate(number, spec.Precision), spec.Currency)
	case KindInteger:
		return fmt.Sprintf("%.0f", math.Round(toFloat(value)))
	case KindPercent:
		precision := spec.Precision
		if precision < 0 {
			precision = 0
		}
		return fmt.Sprintf("%.*f%%", precision, toFloat(value))
	case KindDate:
		layout := spec.Layout
		if layout == "" {
			layout = "2006-01-02"
		}
		switch v := value.(type) {
		case time.Time:
			return v.Format(layout)
		case *time.Time:
			if v == nil {
				return ""
			}
			return v.Format(layout)
		default:
			return fmt.Sprint(value)
		}
	default:
		return defaultFormat(value)
	}
}

func defaultFormat(value any) string {
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

func toFloat(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int8:
		return float64(v)
	case int16:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case uint:
		return float64(v)
	case uint8:
		return float64(v)
	case uint16:
		return float64(v)
	case uint32:
		return float64(v)
	case uint64:
		return float64(v)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err == nil {
			return parsed
		}
		return 0
	default:
		return 0
	}
}
