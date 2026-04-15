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
		// Redis log stores the raw error string; the HTTP layer sanitises
		// before emitting to browsers (see stream_controller.streamClientErrorMessage).
		// Including the raw message here is fine because the event log is
		// server-side only.
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

	return httpdto.StreamEventType(eventType), payload, nil
}

// encodeRunEventFromChunk is the internal shim used by the Redis event-log
// appender. It delegates to BuildPayload and marshals the result so the
// call site in chat_service_impl.go does not need to change.
func encodeRunEventFromChunk(chunk bichatservices.StreamChunk) (string, []byte, error) {
	eventType, payload, err := BuildPayload(chunk)
	if err != nil {
		return "", nil, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", nil, err
	}
	return string(eventType), body, nil
}
