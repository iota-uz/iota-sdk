# Vision Support Setup Guide

Vision support has been implemented for BiChat. This guide shows how to enable it in your application.

## What's Implemented

1. **AttachmentService** (`services/attachment_service.go`)
   - Validates image uploads (JPEG, PNG, GIF, WebP)
   - Enforces size limits (max 20MB per image, max 10 images)
   - Saves to storage with tenant isolation

2. **OpenAI Vision Integration** (`infrastructure/llmproviders/openai_model.go`)
   - Converts attachments to OpenAI multipart content format
   - Uses low-detail mode (85 tokens per image)
   - Supports multiple images per message

3. **GraphQL Upload Handling** (`presentation/graphql/resolvers/resolver.go`)
   - Processes `[Upload!]` attachments in `sendMessage` mutation
   - Calls AttachmentService to validate and save
   - Returns attachment metadata in response

## Setup Steps

### 1. Create FileStorage

```go
import "github.com/iota-uz/iota-sdk/pkg/bichat/storage"

// Local filesystem storage
fileStorage, err := storage.NewLocalFileStorage(
    "/var/lib/bichat/uploads",  // Upload directory
    "https://cdn.example.com",   // CDN URL prefix (or leave empty for local)
)
```

### 2. Create AttachmentService

```go
import bichatservices "github.com/iota-uz/iota-sdk/modules/bichat/services"

attachmentService := bichatservices.NewAttachmentService(fileStorage)
```

### 3. Wire to GraphQL Resolver

When creating your GraphQL resolver, pass the AttachmentService:

```go
import "github.com/iota-uz/iota-sdk/modules/bichat/presentation/graphql/resolvers"

resolver := resolvers.NewResolver(
    app,
    chatService,
    agentService,
    attachmentService, // <-- Vision support
)
```

### 4. Enable Vision in BiChat Config

```go
import "github.com/iota-uz/iota-sdk/modules/bichat"

config := bichat.NewModuleConfig(
    tenantID,
    userID,
    chatRepo,
    model,
    contextPolicy,
    agent,
    bichat.WithVision(true), // <-- Enable vision feature flag
)
```

This passes the feature flag to the React frontend via `window.__BICHAT_CONTEXT__.extensions.features.vision`.

### 5. GraphQL Schema

The schema already supports vision (no changes needed):

```graphql
mutation SendMessage(
  $sessionId: UUID!
  $content: String!
  $attachments: [Upload!]  # Image uploads
) {
  sendMessage(
    sessionId: $sessionId
    content: $content
    attachments: $attachments
  ) {
    userMessage {
      id
      content
      attachments {
        id
        fileName
        mimeType
        sizeBytes
        url
      }
    }
    assistantMessage {
      id
      content
    }
  }
}
```

## Frontend Integration

The React frontend should use `window.__BICHAT_CONTEXT__.extensions.features.vision` to conditionally show image upload UI:

```typescript
const { extensions } = useIotaContext()

if (extensions.features.vision) {
  // Show image upload button in MessageInput
  <ImageUploadButton onUpload={handleUpload} />
}
```

## Validation Rules

- **MIME Types**: image/jpeg, image/png, image/gif, image/webp
- **File Size**: Max 20MB per image
- **Count**: Max 10 images per message
- **Token Cost**: 85 tokens per image (low-detail mode)

## Storage Structure

Images are saved with tenant isolation:

```
/var/lib/bichat/uploads/
├── {tenant-id}/
│   ├── {file-id}.jpg
│   ├── {file-id}.png
│   └── ...
```

## Error Handling

AttachmentService returns structured errors:

- `KindValidation`: Unsupported type, file too large, too many files
- `KindInternal`: Storage failure

Example error response:

```json
{
  "errors": [{
    "message": "unsupported image type: image/bmp (supported: jpeg, png, gif, webp)",
    "extensions": {
      "code": "VALIDATION_ERROR"
    }
  }]
}
```

## Security Considerations

1. **Tenant Isolation**: Files are stored per tenant to prevent cross-tenant access
2. **MIME Type Validation**: Only safe image formats allowed
3. **Size Limits**: Prevents DoS attacks via large uploads
4. **Token Budgeting**: Vision uses low-detail mode to limit costs

## Testing

```bash
# Test attachment service
go test ./modules/bichat/services -run TestAttachmentService -v

# Test vision integration
go test ./modules/bichat/infrastructure/llmproviders -run TestOpenAIVision -v
```

## Troubleshooting

**Images not displaying**: Check CDN URL configuration in FileStorage
**Upload fails**: Check file permissions on upload directory
**Vision not working**: Ensure OpenAI model supports vision (gpt-4-vision-preview, gpt-4o, etc.)
