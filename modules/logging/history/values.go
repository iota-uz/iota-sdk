package history

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

var (
	decimalPattern  = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`)
	integerPattern  = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)
	currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
)

func (t ValueType) valid() bool {
	switch t {
	case ValueString, ValueInteger, ValueDecimal, ValueBoolean, ValueDateTime, ValueDate, ValueMoney:
		return true
	}
	return false
}

// normalize checks a field value against its declared type and returns its canonical JSON,
// so equal values always serialize identically. JSON null is accepted for every type.
func normalize(t ValueType, raw json.RawMessage) (json.RawMessage, error) {
	if !json.Valid(raw) {
		return nil, fmt.Errorf("malformed %s value", t)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("malformed %s value", t)
	}
	if value != nil {
		if err := checkValue(t, value); err != nil {
			return nil, err
		}
	}
	return json.Marshal(value)
}

func checkValue(t ValueType, value any) error {
	switch t {
	case ValueString:
		if _, ok := value.(string); ok {
			return nil
		}
	case ValueInteger:
		if n, ok := value.(json.Number); ok && integerPattern.MatchString(n.String()) {
			return nil
		}
	case ValueDecimal:
		if s, ok := value.(string); ok && decimalPattern.MatchString(s) {
			return nil
		}
	case ValueBoolean:
		if _, ok := value.(bool); ok {
			return nil
		}
	case ValueDateTime:
		if s, ok := value.(string); ok {
			if _, err := time.Parse(time.RFC3339Nano, s); err == nil {
				return nil
			}
		}
	case ValueDate:
		if s, ok := value.(string); ok {
			if _, err := time.Parse(time.DateOnly, s); err == nil {
				return nil
			}
		}
	case ValueMoney:
		if m, ok := value.(map[string]any); ok && len(m) == 2 {
			amount, _ := m["amount"].(string)
			currency, _ := m["currency"].(string)
			if decimalPattern.MatchString(amount) && currencyPattern.MatchString(currency) {
				return nil
			}
		}
	}
	return fmt.Errorf("value is not a valid %s", t)
}
