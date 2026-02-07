package agents

import (
	"context"
)

// TypedTool is a generic Tool implementation that:
// - exposes JSON Schema generated from input type T
// - parses tool input JSON into T
// - calls a typed handler function
//
// This reduces drift between Parameters() and Call().
type TypedTool[T any] struct {
	name        string
	description string
	schema      map[string]any

	handler func(context.Context, T) (string, error)

	onParseError func(ctx context.Context, rawInput string, err error) (string, error)
}

type TypedToolOption[T any] func(*TypedTool[T])

// WithTypedToolSchema overrides the generated JSON Schema.
func WithTypedToolSchema[T any](schema map[string]any) TypedToolOption[T] {
	return func(t *TypedTool[T]) {
		t.schema = schema
	}
}

// WithTypedToolParseErrorHandler sets a handler to convert JSON parsing errors into tool outputs.
// This is useful when tools want to return a structured error string with nil error.
func WithTypedToolParseErrorHandler[T any](fn func(ctx context.Context, rawInput string, err error) (string, error)) TypedToolOption[T] {
	return func(t *TypedTool[T]) {
		t.onParseError = fn
	}
}

// NewTypedTool constructs a Tool backed by a typed handler.
func NewTypedTool[T any](
	name string,
	description string,
	handler func(ctx context.Context, input T) (string, error),
	opts ...TypedToolOption[T],
) Tool {
	tool := &TypedTool[T]{
		name:        name,
		description: description,
		schema:      ToolSchema[T](),
		handler:     handler,
	}
	for _, opt := range opts {
		opt(tool)
	}
	return tool
}

func (t *TypedTool[T]) Name() string { return t.name }

func (t *TypedTool[T]) Description() string { return t.description }

func (t *TypedTool[T]) Parameters() map[string]any { return t.schema }

func (t *TypedTool[T]) Call(ctx context.Context, input string) (string, error) {
	parsed, err := ParseToolInput[T](input)
	if err != nil {
		if t.onParseError != nil {
			return t.onParseError(ctx, input, err)
		}
		return "", err
	}
	return t.handler(ctx, parsed)
}
