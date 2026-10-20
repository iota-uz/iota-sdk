package document

import (
	"encoding/json"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/lens/resource"
)

const DefaultMaxDocumentBytes int64 = 16 << 20

// FrozenDocument owns the canonical wire representation of a document. Its
// bytes are private; Document always transfers a fresh, independent object.
type FrozenDocument struct {
	encoded []byte
}

// Freeze bounds the structural payload before encoding, then retains only the
// immutable wire representation. It does not bound allocations by the producer
// that built source, or the Go overhead of decoded response objects.
func Freeze(source *DashboardDocument, maxBytes int64) (*FrozenDocument, error) {
	if source == nil {
		return nil, fmt.Errorf("document is required")
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxDocumentBytes
	}
	type wireDocument DashboardDocument
	if _, err := resource.Estimate((*wireDocument)(source), maxBytes); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(source)
	if err != nil {
		return nil, fmt.Errorf("freeze lens document: %w", err)
	}
	if int64(len(encoded)) > maxBytes {
		return nil, resource.NewLimitError("document_bytes", int64(len(encoded)), maxBytes)
	}
	return &FrozenDocument{encoded: encoded}, nil
}

func (f *FrozenDocument) Size() int64 {
	if f == nil {
		return 0
	}
	return int64(len(f.encoded))
}

func (f *FrozenDocument) Document() (*DashboardDocument, error) {
	if f == nil {
		return nil, fmt.Errorf("frozen document is required")
	}
	var result DashboardDocument
	if err := json.Unmarshal(f.encoded, &result); err != nil {
		return nil, fmt.Errorf("decode frozen lens document: %w", err)
	}
	return &result, nil
}
