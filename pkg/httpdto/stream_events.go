// Package httpdto provides wire DTO types shared between the backend and the
// TypeScript applet.
package httpdto

// StreamEventType is the name of an SSE event the bichat stream emits.
// These names are the source of truth for both the backend (which writes
// them into the event-log + the SSE `event:` line) and the TS applet
// (which registers EventSource listeners by these names).
type StreamEventType string

const (
	StreamEventChunk         StreamEventType = "chunk"
	StreamEventContent       StreamEventType = "content"
	StreamEventThinking      StreamEventType = "thinking"
	StreamEventToolStart     StreamEventType = "tool_start"
	StreamEventToolEnd       StreamEventType = "tool_end"
	StreamEventTextBlockEnd  StreamEventType = "text_block_end"
	StreamEventSnapshot      StreamEventType = "snapshot"
	StreamEventInterrupt     StreamEventType = "interrupt"
	StreamEventCitation      StreamEventType = "citation"
	StreamEventUsage         StreamEventType = "usage"
	StreamEventPing          StreamEventType = "ping"
	StreamEventStreamStarted StreamEventType = "stream_started"
	StreamEventDone          StreamEventType = "done"
	StreamEventCancelled     StreamEventType = "cancelled"
	StreamEventError         StreamEventType = "error"
	StreamEventFailed        StreamEventType = "failed"
)

// IsTerminal reports whether an event type ends the run. Tailing consumers
// (both server-side and applet-side) MUST settle on any of these.
func IsTerminal(t StreamEventType) bool {
	switch t {
	case StreamEventDone, StreamEventCancelled, StreamEventError, StreamEventFailed:
		return true
	}
	return false
}

// allStreamEventTypes is the authoritative list of every wire event name.
// Keep in sync with the applet's utils/eventNames.ts (there is a
// drift-guard test there).
var allStreamEventTypes = []StreamEventType{
	StreamEventChunk, StreamEventContent, StreamEventThinking,
	StreamEventToolStart, StreamEventToolEnd, StreamEventTextBlockEnd,
	StreamEventSnapshot, StreamEventInterrupt, StreamEventCitation,
	StreamEventUsage, StreamEventPing, StreamEventStreamStarted,
	StreamEventDone, StreamEventCancelled, StreamEventError, StreamEventFailed,
}

// AllStreamEventTypes returns a defensive copy of every wire event name so
// callers cannot mutate the authoritative list. Use slices.Clone when you
// need to iterate in a mutation-prone context.
func AllStreamEventTypes() []StreamEventType {
	return append([]StreamEventType(nil), allStreamEventTypes...)
}
