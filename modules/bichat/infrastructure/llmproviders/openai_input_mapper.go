package llmproviders

import (
	"context"
	"fmt"
	"strings"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

func (m *OpenAIModel) resolveCodeInterpreterFileIDs(ctx context.Context) []string {
	m.mu.RLock()
	resolver := m.artifactResolver
	limit := m.codeInterpreterArtifactLimit
	m.mu.RUnlock()

	if resolver == nil {
		return nil
	}

	sessionID, ok := agents.UseRuntimeSessionID(ctx)
	if !ok {
		return nil
	}

	if limit <= 0 {
		limit = defaultCodeInterpreterFileLimit
	}

	return resolver.ResolveCodeInterpreterFileIDs(ctx, sessionID, limit)
}

// buildInputItems converts types.Message slice to Responses API input items.
func (m *OpenAIModel) buildInputItems(messages []types.Message) responses.ResponseInputParam {
	return m.buildInputItemsWithContext(context.Background(), messages)
}

func (m *OpenAIModel) buildInputItemsWithContext(ctx context.Context, messages []types.Message) responses.ResponseInputParam {
	items := make(responses.ResponseInputParam, 0, len(messages))
	skippedToolCallIDs := make(map[string]struct{})
	imageUploadFileIDs := make(map[int64]string)

	for _, msg := range messages {
		switch msg.Role() {
		case types.RoleSystem:
			items = append(items, responses.ResponseInputItemParamOfMessage(
				msg.Content(),
				responses.EasyInputMessageRoleDeveloper,
			))

		case types.RoleUser:
			if len(msg.Attachments()) > 0 {
				parts := make(responses.ResponseInputMessageContentListParam, 0, 1+len(msg.Attachments()))
				if msg.Content() != "" {
					parts = append(parts, responses.ResponseInputContentParamOfInputText(msg.Content()))
				}
				nonImageNotes := make([]string, 0, len(msg.Attachments()))
				for _, attachment := range msg.Attachments() {
					if strings.HasPrefix(strings.ToLower(strings.TrimSpace(attachment.MimeType)), "image/") {
						if attachment.UploadID != nil && *attachment.UploadID > 0 {
							uploadID := *attachment.UploadID
							fileID := imageUploadFileIDs[uploadID]
							if fileID == "" {
								fileID = m.resolveImageUploadFileID(ctx, uploadID, attachment.FileName, attachment.MimeType)
								if fileID != "" {
									imageUploadFileIDs[uploadID] = fileID
								}
							}
							if fileID != "" {
								parts = append(parts, responses.ResponseInputContentUnionParam{
									OfInputImage: &responses.ResponseInputImageParam{
										FileID: openai.String(fileID),
										Detail: responses.ResponseInputImageDetailLow,
									},
								})
								continue
							}

							nonImageNotes = append(nonImageNotes, fmt.Sprintf(
								"- %s (%s, %d bytes) [uploadId=%d; image embedding unavailable, use artifact_reader]",
								attachment.FileName,
								attachment.MimeType,
								attachment.SizeBytes,
								uploadID,
							))
							continue
						}
						if strings.TrimSpace(attachment.FilePath) == "" {
							nonImageNotes = append(nonImageNotes, fmt.Sprintf("- %s (%s, %d bytes)", attachment.FileName, attachment.MimeType, attachment.SizeBytes))
							continue
						}
						if isLikelyInaccessibleImageURL(attachment.FilePath) {
							nonImageNotes = append(nonImageNotes, fmt.Sprintf(
								"- %s (%s, %d bytes) [image URL inaccessible from provider, use artifact_reader]",
								attachment.FileName,
								attachment.MimeType,
								attachment.SizeBytes,
							))
							continue
						}
						parts = append(parts, responses.ResponseInputContentUnionParam{
							OfInputImage: &responses.ResponseInputImageParam{
								ImageURL: openai.String(attachment.FilePath),
								Detail:   responses.ResponseInputImageDetailLow,
							},
						})
						continue
					}
					nonImageNotes = append(nonImageNotes, fmt.Sprintf("- %s (%s, %d bytes)", attachment.FileName, attachment.MimeType, attachment.SizeBytes))
				}
				if len(nonImageNotes) > 0 {
					parts = append(parts, responses.ResponseInputContentParamOfInputText(
						"Attached files are available in this session. Use artifact_reader to inspect them:\n"+strings.Join(nonImageNotes, "\n"),
					))
				}
				if len(parts) == 0 {
					parts = append(parts, responses.ResponseInputContentParamOfInputText(msg.Content()))
				}
				items = append(items, responses.ResponseInputItemParamOfMessage(
					parts,
					responses.EasyInputMessageRoleUser,
				))
			} else {
				items = append(items, responses.ResponseInputItemParamOfMessage(
					msg.Content(),
					responses.EasyInputMessageRoleUser,
				))
			}

		case types.RoleAssistant:
			if msg.Content() != "" {
				items = append(items, responses.ResponseInputItemParamOfMessage(
					msg.Content(),
					responses.EasyInputMessageRoleAssistant,
				))
			}
			for _, tc := range msg.ToolCalls() {
				callID := strings.TrimSpace(tc.ID)
				callName := strings.TrimSpace(tc.Name)
				if callID == "" || callName == "" {
					m.logger.Warn(context.Background(), "skipping tool call with empty name or ID in buildInputItems", map[string]any{
						"call_id": tc.ID,
						"name":    tc.Name,
					})
					if callID != "" {
						skippedToolCallIDs[callID] = struct{}{}
					}
					continue
				}

				items = append(items, responses.ResponseInputItemParamOfFunctionCall(
					tc.Arguments,
					callID,
					callName,
				))
			}

		case types.RoleTool:
			if msg.ToolCallID() != nil {
				callID := strings.TrimSpace(*msg.ToolCallID())
				if callID == "" {
					continue
				}
				if _, skipped := skippedToolCallIDs[callID]; skipped {
					continue
				}
				items = append(items, responses.ResponseInputItemParamOfFunctionCallOutput(
					callID,
					msg.Content(),
				))
			}
		}
	}

	return items
}
