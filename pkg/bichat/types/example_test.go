package types_test

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// ExampleUserMessage demonstrates creating a user message.
func ExampleUserMessage() {
	msg := types.UserMessage("Hello, assistant!")
	fmt.Printf("Role: %s\n", msg.Role)
	fmt.Printf("Content: %s\n", msg.Content)
	// Output:
	// Role: user
	// Content: Hello, assistant!
}

// ExampleAssistantMessage demonstrates creating an assistant message with tool calls.
func ExampleAssistantMessage() {
	msg := types.AssistantMessage(
		"I'll help you with that.",
		types.WithToolCalls(
			types.ToolCall{
				ID:        "call_123",
				Name:      "search",
				Arguments: `{"query":"bichat"}`,
			},
		),
	)
	fmt.Printf("Role: %s\n", msg.Role)
	fmt.Printf("Tool calls: %d\n", len(msg.ToolCalls))
	// Output:
	// Role: assistant
	// Tool calls: 1
}

// ExampleToolResponse demonstrates creating a tool response message.
func ExampleToolResponse() {
	msg := types.ToolResponse("call_123", "Search results found")
	fmt.Printf("Role: %s\n", msg.Role)
	fmt.Printf("Tool call ID: %s\n", *msg.ToolCallID)
	// Output:
	// Role: tool
	// Tool call ID: call_123
}

// ExampleRole_Valid demonstrates role validation.
func ExampleRole_Valid() {
	validRole := types.RoleUser
	invalidRole := types.Role("invalid")

	fmt.Printf("Valid: %v\n", validRole.Valid())
	fmt.Printf("Invalid: %v\n", invalidRole.Valid())
	// Output:
	// Valid: true
	// Invalid: false
}

// ExampleNotFoundError demonstrates creating a NOT_FOUND error.
func ExampleNotFoundError() {
	err := types.NotFoundError("session", "sess_123")
	fmt.Printf("Error: %v\n", err)
	fmt.Printf("Code: %s\n", err.Code)
	fmt.Printf("Retryable: %v\n", err.Retryable)
	// Output:
	// Error: NOT_FOUND: session not found
	// Code: NOT_FOUND
	// Retryable: false
}

// ExampleValidationError demonstrates creating a VALIDATION error.
func ExampleValidationError() {
	err := types.ValidationError("email", "invalid format")
	fmt.Printf("Error: %v\n", err)
	fmt.Printf("Code: %s\n", err.Code)
	// Output:
	// Error: VALIDATION: validation failed for field 'email': invalid format
	// Code: VALIDATION
}

// ExampleRateLimitError demonstrates creating a RATE_LIMIT error.
func ExampleRateLimitError() {
	err := types.RateLimitError(60 * time.Second)
	fmt.Printf("Code: %s\n", err.Code)
	fmt.Printf("Retryable: %v\n", err.Retryable)
	// Output:
	// Code: RATE_LIMIT
	// Retryable: true
}

// ExampleNewGenerator demonstrates creating and using a generator.
func ExampleNewGenerator() {
	ctx := context.Background()

	// Create a generator that produces numbers 1-5
	gen := types.NewGenerator(ctx, func(ctx context.Context, yield func(int) bool) error {
		for i := 1; i <= 5; i++ {
			if !yield(i) {
				return nil
			}
		}
		return nil
	})

	// Collect all values
	values, err := types.Collect(ctx, gen)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Values: %v\n", values)
	// Output:
	// Values: [1 2 3 4 5]
}

// ExampleAttachment_IsImage demonstrates checking if an attachment is an image.
func ExampleAttachment_IsImage() {
	imageAttachment := &types.Attachment{
		ID:        uuid.New(),
		MessageID: uuid.New(),
		FileName:  "photo.jpg",
		MimeType:  "image/jpeg",
		SizeBytes: 1024,
		CreatedAt: time.Now(),
	}

	pdfAttachment := &types.Attachment{
		ID:        uuid.New(),
		MessageID: uuid.New(),
		FileName:  "document.pdf",
		MimeType:  "application/pdf",
		SizeBytes: 2048,
		CreatedAt: time.Now(),
	}

	fmt.Printf("Image: %v\n", imageAttachment.IsImage())
	fmt.Printf("PDF is image: %v\n", pdfAttachment.IsImage())
	fmt.Printf("PDF is document: %v\n", pdfAttachment.IsDocument())
	// Output:
	// Image: true
	// PDF is image: false
	// PDF is document: true
}
