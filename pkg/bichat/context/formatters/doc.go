// Package formatters provides Formatter implementations for converting
// structured tool payloads into LLM-readable text.
//
// Each formatter corresponds to a codec ID and knows how to render
// one specific payload type. Formatters are registered in a
// FormatterRegistry and invoked by the executor when a StructuredTool
// returns a ToolResult.
package formatters
