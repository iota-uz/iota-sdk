package solid

import (
	"encoding/json"

	"github.com/iota-uz/iota-sdk/pkg/lens/document"
)

func formulaJSON(wire *document.Formula) string {
	if wire == nil {
		return "{}"
	}
	data, err := json.Marshal(wire)
	if err != nil {
		return "{}"
	}
	return string(data)
}
