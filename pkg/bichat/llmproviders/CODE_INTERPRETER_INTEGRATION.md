# OpenAI Code Interpreter Integration

This document explains how the OpenAI Code Interpreter integration works using the Assistants API.

## Architecture Overview

The integration consists of three main components:

1. **AssistantsClient** (`assistants_client.go`) - Manages OpenAI Assistants API calls
2. **CodeInterpreterTool** (`tools/code_interpreter.go`) - Tool interface for code execution
3. **Repository Support** (`postgres_chat_repository.go`) - Stores code interpreter outputs

## How It Works

### 1. Tool Registration

When creating an agent with code interpreter support:

```go
import (
    "github.com/iota-uz/iota-sdk/pkg/bichat/tools"
    "github.com/iota-uz/iota-sdk/pkg/bichat/llmproviders"
    "github.com/iota-uz/iota-sdk/pkg/bichat/storage"
)

// Create file storage for code outputs
fileStorage, err := storage.NewLocalFileStorage(
    "/var/lib/bichat/code-outputs",
    "https://cdn.example.com/code-outputs",
)

// Create Assistants client
assistantsClient := llmproviders.NewAssistantsClient(
    openaiClient,
    fileStorage,
)

// Create code interpreter tool with executor
codeTool := tools.NewCodeInterpreterTool(
    tools.WithAssistantsExecutor(assistantsClient),
)

// Register with agent
agent := bichatagents.NewDefaultBIAgent(
    queryExecutor,
    bichatagents.WithCodeInterpreterTool(codeTool),
)
```

### 2. Code Execution Flow

When the LLM decides to execute Python code:

```
User: "Create a bar chart showing sales by region"
  ↓
LLM calls code_interpreter tool with:
  {
    "description": "Generate sales chart",
    "code": "import matplotlib.pyplot as plt\n..."
  }
  ↓
CodeInterpreterTool.Call() receives request
  ↓
Calls AssistantsClient.ExecuteCodeInterpreter()
  ↓
AssistantsClient workflow:
  1. Creates temporary OpenAI Assistant with code_interpreter enabled
  2. Creates thread and adds user message with code
  3. Runs assistant (polls every 500ms for completion)
  4. Extracts file outputs from assistant response
  5. Downloads files from OpenAI
  6. Stores files using FileStorage
  7. Cleans up assistant and thread
  ↓
Returns CodeInterpreterOutput[] with public URLs
  ↓
Tool returns result JSON with file URLs
  ↓
Repository saves outputs to bichat_code_interpreter_outputs table
  ↓
Frontend displays files (images, CSVs, etc.)
```

### 3. Database Schema

The `bichat_code_interpreter_outputs` table stores generated files:

```sql
CREATE TABLE bichat_code_interpreter_outputs (
    id UUID PRIMARY KEY,
    message_id UUID NOT NULL REFERENCES bichat_messages(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,           -- e.g., "chart.png"
    mime_type VARCHAR(100) NOT NULL,      -- e.g., "image/png"
    url TEXT NOT NULL,                    -- Public URL from FileStorage
    size_bytes BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
```

### 4. File Storage

Code interpreter outputs are stored using the `FileStorage` interface:

- **LocalFileStorage**: Stores files on local filesystem
- **S3Storage** (future): Stores files in Amazon S3
- **NoOpStorage**: For testing (doesn't persist files)

Files are downloaded from OpenAI's temporary storage and uploaded to your configured storage with unique UUIDs.

## Configuration

### Environment Variables

```bash
OPENAI_API_KEY=sk-...           # Required: OpenAI API key
OPENAI_MODEL=gpt-4o             # Optional: Model for code interpreter
```

### Storage Configuration

```go
// Local filesystem storage
storage, err := storage.NewLocalFileStorage(
    "/var/lib/bichat/files",           // Base directory
    "https://example.com/bichat/files", // Base URL for downloads
)

// S3 storage (future)
// storage, err := storage.NewS3FileStorage(bucket, region, baseURL)
```

### Module Configuration

```go
cfg := bichat.NewModuleConfig(
    composables.UseTenantID,
    composables.UseUserID,
    chatRepo,
    model,
    bichat.DefaultContextPolicy(),
    parentAgent,
    bichat.WithCodeInterpreter(true), // Enable code interpreter
)
```

## Supported Python Libraries

The OpenAI code interpreter sandbox supports:

- **Data Analysis**: pandas, numpy, scipy, scikit-learn
- **Visualization**: matplotlib, seaborn, plotly
- **File I/O**: Reading/writing CSV, Excel, JSON, etc.
- **Math**: sympy, statistics
- **Standard Library**: datetime, math, random, etc.

## Output File Types

Common file outputs:

- **Images**: PNG (matplotlib plots, seaborn charts)
- **Data**: CSV (pandas DataFrames), Excel (xlsx)
- **Documents**: PDF (reportlab)
- **Other**: JSON, TXT

## Error Handling

```go
outputs, err := assistantsClient.ExecuteCodeInterpreter(ctx, messageID, userMessage)
if err != nil {
    // Handle errors:
    // - API failures (network, auth)
    // - Code execution errors (syntax, runtime)
    // - Timeout (5 minute max)
    // - Storage failures (disk full, permissions)
    return err
}
```

## Security Considerations

1. **Sandboxed Execution**: Code runs in OpenAI's isolated containers
2. **No External Access**: Cannot make network requests or access external APIs
3. **Temporary Environment**: Fresh container per execution
4. **File Cleanup**: OpenAI files are automatically deleted after 24 hours
5. **Local Storage**: Downloaded files stored with unique UUIDs to prevent overwrites

## Performance

- **Execution Time**: Typically 2-10 seconds for simple code
- **Timeout**: 5 minutes maximum per execution
- **Polling Interval**: 500ms status checks
- **File Size Limit**: OpenAI enforces limits on generated files

## Testing

### Unit Tests

```bash
# Test code interpreter tool
go test ./pkg/bichat/tools -run TestCodeInterpreter

# Test assistants client
go test ./pkg/bichat/llmproviders -run TestAssistantsClient
```

### Integration Tests

```bash
# Requires OPENAI_API_KEY
export OPENAI_API_KEY=sk-...
go test -v -tags=integration ./pkg/bichat/llmproviders/...
```

### Example Test

```go
func TestCodeInterpreter(t *testing.T) {
    client := openai.NewClient(apiKey)
    storage := storage.NewNoOpFileStorage()
    assistants := llmproviders.NewAssistantsClient(client, storage)

    outputs, err := assistants.ExecuteCodeInterpreter(
        context.Background(),
        uuid.New(),
        "Create a bar chart: North=100, South=150, East=120, West=90",
    )

    assert.NoError(t, err)
    assert.NotEmpty(t, outputs)
    assert.Equal(t, "chart.png", outputs[0].Name)
}
```

## Troubleshooting

### Common Issues

**Code Execution Fails**:
- Check OpenAI API key is valid
- Verify model supports code interpreter (gpt-4o, gpt-4-turbo)
- Check Python syntax in generated code
- Review OpenAI status page for outages

**File Download Fails**:
- Ensure FileStorage is configured correctly
- Check disk space and permissions
- Verify network connectivity to OpenAI

**Timeout Errors**:
- Long-running code may exceed 5-minute limit
- Simplify code or split into multiple executions
- Check OpenAI API status

**Storage Issues**:
- Verify base directory exists and is writable
- Check disk space
- Ensure base URL is accessible

## Future Enhancements

1. **File Upload Support**: Allow uploading CSV/Excel for analysis
2. **Persistent Sessions**: Reuse assistants across multiple requests
3. **Streaming Support**: Stream code output as it executes
4. **Custom Libraries**: Support for additional Python packages
5. **S3 Storage Backend**: Direct upload to cloud storage

## References

- [OpenAI Assistants API Documentation](https://platform.openai.com/docs/assistants/overview)
- [Code Interpreter Documentation](https://platform.openai.com/docs/assistants/tools/code-interpreter)
- [go-openai Library](https://github.com/sashabaranov/go-openai)
