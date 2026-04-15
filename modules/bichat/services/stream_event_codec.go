// Package services provides this package.
package services

import (
	"encoding/json"
	"strings"

	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/httpdto"
)

// BuildPayload converts an in-memory StreamChunk into the typed
// httpdto.StreamEventType + httpdto.StreamChunkPayload pair. It is the
// single encoding point for all chunk types; callers that need a []byte
// (e.g. the Redis event-log appender) marshal the returned struct themselves.
//
// When the chunk type is empty we default to StreamEventChunk ("chunk") to
// keep parity with the controller's fallback.
func BuildPayload(chunk bichatservices.StreamChunk) (httpdto.StreamEventType, httpdto.StreamChunkPayload, error) {
	payload := httpdto.StreamChunkPayload{
		Type:         string(chunk.Type),
		Content:      chunk.Content,
		Citation:     chunk.Citation,
		Usage:        chunk.Usage,
		GenerationMs: chunk.GenerationMs,
		Timestamp:    chunk.Timestamp.UnixMilli(),
		RunID:        chunk.RunID,
	}
	if chunk.Tool != nil {
		toolPayload := &httpdto.ToolEventPayload{
			CallID:     chunk.Tool.CallID,
			Name:       chunk.Tool.Name,
			AgentName:  chunk.Tool.AgentName,
			Arguments:  chunk.Tool.Arguments,
			Result:     chunk.Tool.Result,
			DurationMs: chunk.Tool.DurationMs,
		}
		if chunk.Tool.Error != nil {
			toolPayload.Error = chunk.Tool.Error.Error()
		}
		payload.Tool = toolPayload
	}
	if chunk.Interrupt != nil {
		questions := make([]httpdto.InterruptQuestionPayload, 0, len(chunk.Interrupt.Questions))
		for _, q := range chunk.Interrupt.Questions {
			options := make([]httpdto.InterruptQuestionOptionPayload, 0, len(q.Options))
			for _, opt := range q.Options {
				options = append(options, httpdto.InterruptQuestionOptionPayload{
					ID:    opt.ID,
					Label: opt.Label,
				})
			}
			questions = append(questions, httpdto.InterruptQuestionPayload{
				ID:      q.ID,
				Text:    q.Text,
				Type:    string(q.Type),
				Options: options,
			})
		}
		payload.Interrupt = &httpdto.InterruptEventPayload{
			CheckpointID:       chunk.Interrupt.CheckpointID,
			AgentName:          chunk.Interrupt.AgentName,
			ProviderResponseID: chunk.Interrupt.ProviderResponseID,
			Questions:          questions,
		}
	}
	if chunk.Error != nil {
		// Store the raw error string here; encodeRunEventFromChunk applies
		// sanitizeChunkError before persisting to Redis so replay paths never
		// expose provider internals. The live HTTP path is sanitised by
		// stream_controller.streamClientErrorMessage.
		payload.Error = chunk.Error.Error()
	}
	if chunk.Snapshot != nil {
		payload.Snapshot = &httpdto.StreamSnapshotPayload{
			PartialContent:  chunk.Snapshot.PartialContent,
			PartialMetadata: chunk.Snapshot.PartialMetadata,
		}
	}
	if chunk.Type == bichatservices.ChunkTypeTextBlockEnd {
		seq := chunk.TextBlockSeq
		payload.TextBlockSeq = &seq
	}

	eventType := strings.TrimSpace(string(chunk.Type))
	if eventType == "" {
		eventType = string(httpdto.StreamEventChunk)
	}
	// Overwrite payload.Type with the resolved event name (including the
	// "chunk" fallback) so the SSE event: line and the JSON body.type field
	// always agree for persisted/replayed events.
	payload.Type = eventType

	return httpdto.StreamEventType(eventType), payload, nil
}

// sanitizeChunkError returns a safe error string for storage in the Redis
// event log so that replayed events via GET /stream/events cannot leak raw
// internal error text (e.g. provider stack traces) to browsers.
//
// The sanitization logic mirrors StreamController.streamClientErrorMessage.
// Keep both in sync when updating error-handling behaviour.
func sanitizeChunkError(chunk bichatservices.StreamChunk) string {
	if chunk.Error == nil {
		return ""
	}
	if chunk.Type != bichatservices.ChunkTypeError {
		// Non-error chunk types (e.g. tool errors) carry localised tool info;
		// preserve them so the applet can display tool-level error detail.
		return chunk.Error.Error()
	}
	// For terminal error chunks, return a generic safe message to avoid
	// leaking provider internals. The applet only needs to know the run failed.
	return "An error occurred while processing your request"
}

// encodeRunEventFromChunk is the internal shim used by the Redis event-log
// appender. It delegates to BuildPayload, applies error sanitisation so the
// stored JSON never contains raw internal error strings, then marshals the
// result. Only sanitised text is written to Redis; the HTTP layer uses an
// equivalent sanitiser (StreamController.streamClientErrorMessage) on the
// live stream path.
func encodeRunEventFromChunk(chunk bichatservices.StreamChunk) (string, []byte, error) {
	eventType, payload, err := BuildPayload(chunk)
	if err != nil {
		return "", nil, err
	}
	// Sanitise before storing so replay over GET /stream/events cannot expose
	// raw provider error strings. Mirrors the controller's post-encode step.
	if chunk.Error != nil {
		payload.Error = sanitizeChunkError(chunk)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", nil, err
	}
	return string(eventType), body, nil
}
