package serrors

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/iota-uz/go-i18n/v2/i18n"
)

// MessageFromConfig explicitly authorizes a legacy localization configuration.
// It rejects arbitrary objects and custom template execution. String arguments
// are text; callers must use Reference for nested localized labels.
func MessageFromConfig(cfg *i18n.LocalizeConfig) (Message, error) {
	if cfg == nil {
		return Message{}, nil
	}
	if cfg.TemplateParser != nil || len(cfg.Funcs) > 0 {
		return Message{}, fmt.Errorf("unsupported localization template execution")
	}
	m := Message{ID: cfg.MessageID}
	if cfg.DefaultMessage != nil {
		m.ID = cfg.DefaultMessage.ID
		if m.ID == "" {
			m.Text = cfg.DefaultMessage.Other
		}
	}
	if cfg.PluralCount != nil {
		value, err := integralCount(cfg.PluralCount)
		if err != nil {
			return Message{}, err
		}
		m.Count = &value
	}
	if cfg.TemplateData == nil {
		return m, nil
	}
	data := reflect.ValueOf(cfg.TemplateData)
	if data.Kind() != reflect.Map || data.Type().Key().Kind() != reflect.String {
		return Message{}, fmt.Errorf("localization arguments must be a string-keyed map")
	}
	m.Args = make(map[string]Value, data.Len())
	iterator := data.MapRange()
	for iterator.Next() {
		key := iterator.Key().String()
		value, err := localizationValue(iterator.Value().Interface())
		if err != nil {
			return Message{}, fmt.Errorf("localization argument %q: %w", key, err)
		}
		m.Args[key] = value
	}
	return m, nil
}

func integralCount(value any) (int64, error) {
	v := reflect.ValueOf(value)
	switch {
	case v.Kind() >= reflect.Int && v.Kind() <= reflect.Int64:
		return v.Int(), nil
	case v.Kind() >= reflect.Uint && v.Kind() <= reflect.Uint64:
		count := v.Uint()
		if count <= ^uint64(0)>>1 {
			return int64(count), nil
		}
	case v.Kind() == reflect.String:
		count, err := strconv.ParseInt(v.String(), 10, 64)
		if err == nil {
			return count, nil
		}
	}
	return 0, fmt.Errorf("plural count must be an integer")
}

func localizationValue(value any) (Value, error) {
	switch v := value.(type) {
	case Value:
		return cloneValues(map[string]Value{"v": v})["v"], nil
	case Message:
		return Reference(v), nil
	}
	v := reflect.ValueOf(value)
	switch {
	case v.Kind() == reflect.String:
		return Text(v.String()), nil
	case v.Kind() == reflect.Bool:
		return Boolean(v.Bool()), nil
	case v.Kind() >= reflect.Int && v.Kind() <= reflect.Int64:
		return Number(v.Int()), nil
	case v.Kind() >= reflect.Uint && v.Kind() <= reflect.Uint64:
		count, err := integralCount(value)
		if err == nil {
			return Number(count), nil
		}
	}
	return Value{}, fmt.Errorf("unsupported localization value type %T", value)
}
