package ui

import "encoding/json"

// RPCResponseForTest is the decoded /_ext/rpc envelope.
type RPCResponseForTest struct {
	OK    bool            `json:"ok"`
	Value json.RawMessage `json:"value"`
	Error *struct {
		Kind    string          `json:"kind"`
		Code    string          `json:"code"`
		Message string          `json:"message"`
		Params  json.RawMessage `json:"params"`
		Issues  []struct {
			Code   string          `json:"code"`
			Phase  string          `json:"phase"`
			Source string          `json:"source"`
			Path   json.RawMessage `json:"path"`
			Params json.RawMessage `json:"params"`
		} `json:"issues"`
	} `json:"error"`
}

// CleanAssetPath exposes the asset path confinement for tests.
func CleanAssetPath(p string) (string, bool) { return cleanAssetPath(p) }
