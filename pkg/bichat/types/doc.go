// Package types provides canonical types for the BiChat library.
//
// This package defines the core data structures used throughout the BiChat
// agent framework, including messages, roles, tools, attachments, citations,
// and error handling. These types serve as the single source of truth to
// eliminate type duplication across the codebase.
//
// Key Types:
//   - Message: Core message structure with role, content, and metadata
//   - Role: Enumeration of message roles (system, user, assistant, tool)
//   - ToolCall: Represents a tool invocation request
//   - ToolCallResult: Represents the result of a tool execution
//   - Attachment: File attachments associated with messages
//   - Citation: Source citations for generated content
//   - Generator: Context-aware iterator interface for streaming operations
//   - Error: Rich error type with structured error codes and metadata
//   - TokenUsage: Token consumption metrics for LLM operations
package types
