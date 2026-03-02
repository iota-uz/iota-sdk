// Package streaming provides this package.
package streaming

import (
	"time"

	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
)

// TerminalChunk builds a terminal stream chunk according to standard streaming policy.
func TerminalChunk(err error, generationMs int64) bichatservices.StreamChunk {
	if err != nil {
		return bichatservices.StreamChunk{
			Type:      bichatservices.ChunkTypeError,
			Error:     err,
			Timestamp: time.Now(),
		}
	}
	return bichatservices.StreamChunk{
		Type:         bichatservices.ChunkTypeDone,
		GenerationMs: generationMs,
		Timestamp:    time.Now(),
	}
}
