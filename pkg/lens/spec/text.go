package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type Text struct {
	Value        string
	Translations map[string]string
}

func LiteralText(value string) Text {
	return Text{Value: value}
}

func (t *Text) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*t = Text{}
		return nil
	}

	var single string
	if err := json.Unmarshal(trimmed, &single); err == nil {
		*t = Text{Value: single}
		return nil
	}

	var translations map[string]string
	if err := json.Unmarshal(trimmed, &translations); err != nil {
		return fmt.Errorf("text must be a string or locale map: %w", err)
	}

	normalized := make(map[string]string, len(translations))
	for locale, value := range translations {
		key := normalizeLocale(locale)
		if key == "" {
			continue
		}
		normalized[key] = value
	}
	*t = Text{Translations: normalized}
	return nil
}

func (t Text) Resolve(locale string) string {
	if len(t.Translations) == 0 {
		return t.Value
	}

	normalized := normalizeLocale(locale)
	if normalized != "" {
		if value, ok := t.Translations[normalized]; ok && strings.TrimSpace(value) != "" {
			return value
		}
		if base, _, ok := strings.Cut(normalized, "-"); ok {
			if value, exists := t.Translations[base]; exists && strings.TrimSpace(value) != "" {
				return value
			}
		}
	}

	for _, fallback := range []string{"en", "ru", "uz", "oz"} {
		if value, ok := t.Translations[fallback]; ok && strings.TrimSpace(value) != "" {
			return value
		}
	}

	keys := make([]string, 0, len(t.Translations))
	for key := range t.Translations {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if value := t.Translations[key]; strings.TrimSpace(value) != "" {
			return value
		}
	}

	return t.Value
}

type Duration time.Duration

func (d *Duration) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*d = 0
		return nil
	}

	var asString string
	if err := json.Unmarshal(trimmed, &asString); err == nil {
		parsed, err := time.ParseDuration(strings.TrimSpace(asString))
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", asString, err)
		}
		*d = Duration(parsed)
		return nil
	}

	var asNumber float64
	if err := json.Unmarshal(trimmed, &asNumber); err != nil {
		return fmt.Errorf("duration must be a string or number: %w", err)
	}
	*d = Duration(time.Duration(asNumber))
	return nil
}

func (d Duration) Std() time.Duration {
	return time.Duration(d)
}

func normalizeLocale(locale string) string {
	trimmed := strings.TrimSpace(locale)
	if trimmed == "" {
		return ""
	}
	return strings.ToLower(strings.ReplaceAll(trimmed, "_", "-"))
}
