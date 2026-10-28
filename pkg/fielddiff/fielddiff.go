// Package fielddiff describes field-level changes shared by MCP change plans and business history.
package fielddiff

import (
	"encoding/json"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type State string

const (
	StateValue    State = "value"
	StateAbsent   State = "absent"
	StateRedacted State = "redacted"
)

// FieldValue tells a present value (JSON null included) apart from an absent field and a hidden one.
type FieldValue struct {
	State State
	Value json.RawMessage
}

type Diff struct {
	Field  string     `json:"field"`
	Before FieldValue `json:"before"`
	After  FieldValue `json:"after"`
}

func Present(v json.RawMessage) FieldValue {
	if len(v) == 0 {
		v = json.RawMessage("null")
	}
	return FieldValue{State: StateValue, Value: v}
}

func Absent() FieldValue {
	return FieldValue{State: StateAbsent}
}

func Redacted() FieldValue {
	return FieldValue{State: StateRedacted}
}

func (f FieldValue) MarshalJSON() ([]byte, error) {
	switch f.State {
	case StateValue:
		v := f.Value
		if len(v) == 0 {
			v = json.RawMessage("null")
		}
		return json.Marshal(struct {
			State State           `json:"state"`
			Value json.RawMessage `json:"value"`
		}{f.State, v})
	case StateAbsent, StateRedacted:
		return json.Marshal(struct {
			State State `json:"state"`
		}{f.State})
	default:
		return nil, serrors.NewInvalid("unknown field value state " + string(f.State))
	}
}

func (f *FieldValue) UnmarshalJSON(data []byte) error {
	const op serrors.Op = "fielddiff.FieldValue.UnmarshalJSON"
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return serrors.Wrap(op, err)
	}
	var state State
	if err := json.Unmarshal(raw["state"], &state); err != nil {
		return serrors.NewInvalid("field value state is required")
	}
	value, hasValue := raw["value"]
	for key := range raw {
		if key != "state" && key != "value" {
			return serrors.NewInvalid("unknown field value key " + key)
		}
	}
	switch state {
	case StateValue:
		if !hasValue {
			return serrors.NewInvalid("field value with state value requires value")
		}
		*f = FieldValue{State: state, Value: value}
	case StateAbsent, StateRedacted:
		if hasValue {
			return serrors.NewInvalid("field value with state " + string(state) + " must not carry value")
		}
		*f = FieldValue{State: state}
	default:
		return serrors.NewInvalid("unknown field value state " + string(state))
	}
	return nil
}
