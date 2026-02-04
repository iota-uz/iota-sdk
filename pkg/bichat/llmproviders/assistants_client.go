package llmproviders

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/storage"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/sashabaranov/go-openai"
)

// OpenAIAssistantsAPI defines the subset of OpenAI API needed for Assistants.
// This interface allows for easier testing via mocks.
type OpenAIAssistantsAPI interface {
	CreateAssistant(ctx context.Context, request openai.AssistantRequest) (openai.Assistant, error)
	DeleteAssistant(ctx context.Context, assistantID string) (openai.AssistantDeleteResponse, error)
	CreateThread(ctx context.Context, request openai.ThreadRequest) (openai.Thread, error)
	DeleteThread(ctx context.Context, threadID string) (openai.ThreadDeleteResponse, error)
	CreateMessage(ctx context.Context, threadID string, request openai.MessageRequest) (openai.Message, error)
	CreateRun(ctx context.Context, threadID string, request openai.RunRequest) (openai.Run, error)
	RetrieveRun(ctx context.Context, threadID, runID string) (openai.Run, error)
	ListMessage(ctx context.Context, threadID string, limit *int, order *string, after *string, before *string, runID *string) (openai.MessagesList, error)
	GetFile(ctx context.Context, fileID string) (openai.File, error)
	GetFileContent(ctx context.Context, fileID string) (io.ReadCloser, error)
}

// AssistantsClient manages OpenAI Assistants API for code interpreter execution.
// It creates temporary assistants, executes Python code, downloads outputs, and cleans up.
type AssistantsClient struct {
	client      OpenAIAssistantsAPI
	fileStorage storage.FileStorage
}

// NewAssistantsClient creates a new Assistants API client.
//
// Parameters:
//   - client: OpenAI client for API calls (must satisfy OpenAIAssistantsAPI interface)
//   - fileStorage: Storage backend for saving downloaded files
//
// Example:
//
//	storage, _ := storage.NewLocalFileStorage("/var/lib/bichat/files", "https://example.com/files")
//	assistants := NewAssistantsClient(openaiClient, storage)
func NewAssistantsClient(client OpenAIAssistantsAPI, fileStorage storage.FileStorage) *AssistantsClient {
	return &AssistantsClient{
		client:      client,
		fileStorage: fileStorage,
	}
}

// ExecuteCodeInterpreter executes Python code using OpenAI Assistants API.
//
// Workflow:
//  1. Create temporary assistant with code_interpreter enabled
//  2. Create thread and add user message with code
//  3. Run assistant and poll for completion
//  4. Extract file outputs from run results
//  5. Download files from OpenAI
//  6. Store files using FileStorage
//  7. Clean up assistant and thread
//
// Parameters:
//   - ctx: Context for cancellation
//   - messageID: Message ID for associating outputs
//   - userMessage: User's message (may contain code request)
//
// Returns:
//   - outputs: List of generated files with URLs
//   - error: Any error during execution
func (c *AssistantsClient) ExecuteCodeInterpreter(
	ctx context.Context,
	messageID uuid.UUID,
	userMessage string,
) ([]types.CodeInterpreterOutput, error) {
	const op serrors.Op = "AssistantsClient.ExecuteCodeInterpreter"

	// Step 1: Create temporary assistant with code_interpreter
	instructions := "You are a helpful data analyst. Execute Python code to analyze data and generate visualizations."
	assistant, err := c.client.CreateAssistant(ctx, openai.AssistantRequest{
		Model: openai.GPT4o, // Use GPT-4o for code interpreter
		Tools: []openai.AssistantTool{
			{Type: openai.AssistantToolTypeCodeInterpreter},
		},
		Instructions: &instructions,
	})
	if err != nil {
		return nil, serrors.E(op, err, "failed to create assistant")
	}

	// Ensure assistant cleanup
	defer func() {
		_, _ = c.client.DeleteAssistant(context.Background(), assistant.ID)
	}()

	// Step 2: Create thread
	thread, err := c.client.CreateThread(ctx, openai.ThreadRequest{})
	if err != nil {
		return nil, serrors.E(op, err, "failed to create thread")
	}

	// Ensure thread cleanup
	defer func() {
		_, _ = c.client.DeleteThread(context.Background(), thread.ID)
	}()

	// Step 3: Add user message to thread
	_, err = c.client.CreateMessage(ctx, thread.ID, openai.MessageRequest{
		Role:    string(openai.ChatMessageRoleUser),
		Content: userMessage,
	})
	if err != nil {
		return nil, serrors.E(op, err, "failed to create message")
	}

	// Step 4: Run assistant
	run, err := c.client.CreateRun(ctx, thread.ID, openai.RunRequest{
		AssistantID: assistant.ID,
	})
	if err != nil {
		return nil, serrors.E(op, err, "failed to create run")
	}

	// Step 5: Poll for completion
	run, err = c.pollRunCompletion(ctx, thread.ID, run.ID)
	if err != nil {
		return nil, serrors.E(op, err, "run polling failed")
	}

	// Check run status
	if run.Status != openai.RunStatusCompleted {
		return nil, serrors.E(op, fmt.Errorf("run failed with status: %s", run.Status))
	}

	// Step 6: Extract and download file outputs
	outputs, err := c.extractFileOutputs(ctx, messageID, thread.ID)
	if err != nil {
		return nil, serrors.E(op, err, "failed to extract file outputs")
	}

	return outputs, nil
}

// pollRunCompletion polls the run status until completion or timeout.
// Checks status every 500ms with a maximum timeout of 5 minutes.
func (c *AssistantsClient) pollRunCompletion(ctx context.Context, threadID, runID string) (openai.Run, error) {
	const op serrors.Op = "AssistantsClient.pollRunCompletion"

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(5 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return openai.Run{}, serrors.E(op, ctx.Err(), "context cancelled")

		case <-timeout:
			return openai.Run{}, serrors.E(op, "timeout waiting for run completion")

		case <-ticker.C:
			run, err := c.client.RetrieveRun(ctx, threadID, runID)
			if err != nil {
				return openai.Run{}, serrors.E(op, err, "failed to retrieve run")
			}

			// Terminal states
			switch run.Status {
			case openai.RunStatusCompleted, openai.RunStatusFailed,
				openai.RunStatusCancelled, openai.RunStatusExpired:
				return run, nil
			case openai.RunStatusQueued, openai.RunStatusInProgress,
				openai.RunStatusRequiresAction, openai.RunStatusCancelling, openai.RunStatusIncomplete:
				// Continue polling
			default:
				// Unknown status, continue polling
			}
		}
	}
}

// extractFileOutputs extracts file outputs from thread messages.
func (c *AssistantsClient) extractFileOutputs(
	ctx context.Context,
	messageID uuid.UUID,
	threadID string,
) ([]types.CodeInterpreterOutput, error) {
	const op serrors.Op = "AssistantsClient.extractFileOutputs"

	// List messages in thread (most recent first)
	messages, err := c.client.ListMessage(ctx, threadID, nil, nil, nil, nil, nil)
	if err != nil {
		return nil, serrors.E(op, err, "failed to list messages")
	}

	var outputs []types.CodeInterpreterOutput
	seenFileIDs := make(map[string]bool) // Track file IDs to avoid duplicates

	// Iterate through messages to find assistant responses with file outputs
	for _, msg := range messages.Messages {
		// Only process assistant messages
		if msg.Role != string(openai.ChatMessageRoleAssistant) {
			continue
		}

		// Check message content for file outputs
		for _, content := range msg.Content {
			// Handle image_file content type
			if content.ImageFile != nil && content.ImageFile.FileID != "" {
				fileID := content.ImageFile.FileID
				if !seenFileIDs[fileID] {
					seenFileIDs[fileID] = true
					output, err := c.downloadAndStoreFile(ctx, messageID, fileID)
					if err != nil {
						// Log error but continue processing other files
						continue
					}
					outputs = append(outputs, output)
				}
			}

			// TODO: Handle text content with file annotations
			// The go-openai library structure for annotations needs to be verified
			// For now, we focus on image_file which is commonly used for charts/plots
		}
	}

	return outputs, nil
}

// downloadAndStoreFile downloads a file from OpenAI and stores it using FileStorage.
func (c *AssistantsClient) downloadAndStoreFile(
	ctx context.Context,
	messageID uuid.UUID,
	fileID string,
) (types.CodeInterpreterOutput, error) {
	const op serrors.Op = "AssistantsClient.downloadAndStoreFile"

	// Retrieve file metadata from OpenAI
	fileInfo, err := c.client.GetFile(ctx, fileID)
	if err != nil {
		return types.CodeInterpreterOutput{}, serrors.E(op, err, "failed to get file info")
	}

	// Download file content from OpenAI
	fileContent, err := c.client.GetFileContent(ctx, fileID)
	if err != nil {
		return types.CodeInterpreterOutput{}, serrors.E(op, err, "failed to download file")
	}

	// Determine MIME type from filename
	mimeType := "application/octet-stream"
	if fileInfo.FileName != "" {
		ext := filepath.Ext(fileInfo.FileName)
		if detectedMime := mime.TypeByExtension(ext); detectedMime != "" {
			mimeType = detectedMime
		}
	}

	// Try to detect MIME type from content if filename detection failed
	if mimeType == "application/octet-stream" {
		buffer := make([]byte, 512)
		n, _ := io.ReadFull(fileContent, buffer)
		if n > 0 {
			detectedMime := http.DetectContentType(buffer[:n])
			if detectedMime != "" {
				mimeType = detectedMime
			}
		}
		// Reset reader by creating new one (we consumed some bytes)
		fileContent, err = c.client.GetFileContent(ctx, fileID)
		if err != nil {
			return types.CodeInterpreterOutput{}, serrors.E(op, err, "failed to re-download file")
		}
	}

	// Store file using FileStorage
	metadata := storage.FileMetadata{
		ContentType: mimeType,
		Size:        int64(fileInfo.Bytes),
	}

	url, err := c.fileStorage.Save(ctx, fileInfo.FileName, fileContent, metadata)
	if err != nil {
		return types.CodeInterpreterOutput{}, serrors.E(op, err, "failed to save file")
	}

	// Create CodeInterpreterOutput
	output := types.CodeInterpreterOutput{
		ID:        uuid.New(),
		MessageID: messageID,
		Name:      fileInfo.FileName,
		MimeType:  mimeType,
		URL:       url,
		Size:      int64(fileInfo.Bytes),
		CreatedAt: time.Now(),
	}

	return output, nil
}
