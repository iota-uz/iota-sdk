package mcp

import (
	"encoding/json"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Result is {ok: true, data} or {ok: false, error}; transports and the Python
// runtime pass it through unchanged.
type Result struct {
	OK    bool
	Data  json.RawMessage
	Error *Failure
}

func Success(data any) (Result, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return Result{}, err
	}
	return Result{OK: true, Data: raw}, nil
}

func Failed(f *Failure) Result { return Result{Error: f} }

func (r Result) MarshalJSON() ([]byte, error) {
	if r.OK {
		data := r.Data
		if len(data) == 0 {
			data = json.RawMessage("null")
		}
		return json.Marshal(struct {
			OK   bool            `json:"ok"`
			Data json.RawMessage `json:"data"`
		}{true, data})
	}
	return json.Marshal(struct {
		OK    bool     `json:"ok"`
		Error *Failure `json:"error"`
	}{false, r.Error})
}

func (r *Result) UnmarshalJSON(b []byte) error {
	var raw struct {
		OK    bool            `json:"ok"`
		Data  json.RawMessage `json:"data"`
		Error *Failure        `json:"error"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if !raw.OK && raw.Error == nil {
		return serrors.New(serrors.Invalid, "failed result without error")
	}
	*r = Result{OK: raw.OK, Data: raw.Data, Error: raw.Error}
	return nil
}
