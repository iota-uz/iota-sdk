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

// userContentPartKind describes a single element of normalized user message content.
const (
	userPartText = iota
	userPartImageFileID
	userPartImageURL
	userPartNoteLine
)

type userContentPart struct {
	kind  int
	value string
}

// resolveImageUploadsForMessages resolves all image upload IDs in the message list up front.
// Returns uploadID -> provider fileID. Resolution failures are omitted (and will be treated as unavailable in content normalization).
func (m *OpenAIModel) resolveImageUploadsForMessages(ctx context.Context, messages []types.Message) map[int64]string {
	out := make(map[int64]string)
	for _, msg := range messages {
		if msg.Role() != types.RoleUser {
			continue
		}
		for _, att := range msg.Attachments() {
			if att.UploadID == nil || *att.UploadID <= 0 {
				continue
			}
			id := *att.UploadID
			if _, ok := out[id]; ok {
				continue
			}
			if fileID := m.resolveImageUploadFileID(ctx, id, att.FileName, att.MimeType); fileID != "" {
				out[id] = fileID
			}
		}
	}
	return out
}

// validToolCallIDsFromMessages returns the set of tool call IDs that have both non-empty ID and name.
// Invalid tool calls are logged; their outputs will be skipped when mapping.
func (m *OpenAIModel) validToolCallIDsFromMessages(messages []types.Message) map[string]struct{} {
	valid := make(map[string]struct{})
	for _, msg := range messages {
		if msg.Role() != types.RoleAssistant {
			continue
		}
		for _, tc := range msg.ToolCalls() {
			callID := strings.TrimSpace(tc.ID)
			callName := strings.TrimSpace(tc.Name)
			if callID != "" && callName != "" {
				valid[callID] = struct{}{}
				continue
			}
			m.logger.Warn(context.Background(), "skipping tool call with empty name or ID in buildInputItems", map[string]any{
				"call_id": tc.ID,
				"name":    tc.Name,
			})
		}
	}
	return valid
}

// normalizeUserContent turns a user message and resolved upload map into an ordered list of content parts.
// All "can we send this to the API?" decisions happen here; the mapper then only translates part kinds to API types.
func normalizeUserContent(msg types.Message, resolved map[int64]string) []userContentPart {
	attachments := msg.Attachments()
	if len(attachments) == 0 {
		if msg.Content() == "" {
			return nil
		}
		return []userContentPart{{kind: userPartText, value: msg.Content()}}
	}

	var parts []userContentPart
	if msg.Content() != "" {
		parts = append(parts, userContentPart{kind: userPartText, value: msg.Content()})
	}

	var noteLines []string
	for _, att := range attachments {
		mime := strings.ToLower(strings.TrimSpace(att.MimeType))
		isImage := strings.HasPrefix(mime, "image/")
		baseNote := fmt.Sprintf("- %s (%s, %d bytes)", att.FileName, att.MimeType, att.SizeBytes)

		if isImage {
			if att.UploadID != nil && *att.UploadID > 0 {
				uploadID := *att.UploadID
				if fileID := resolved[uploadID]; fileID != "" {
					parts = append(parts, userContentPart{kind: userPartImageFileID, value: fileID})
					continue
				}
				noteLines = append(noteLines, fmt.Sprintf("%s [uploadId=%d; image embedding unavailable, use artifact_reader]", baseNote, uploadID))
				continue
			}
			if strings.TrimSpace(att.FilePath) == "" {
				noteLines = append(noteLines, baseNote)
				continue
			}
			if isLikelyInaccessibleImageURL(att.FilePath) {
				noteLines = append(noteLines, baseNote+" [image URL inaccessible from provider, use artifact_reader]")
				continue
			}
			parts = append(parts, userContentPart{kind: userPartImageURL, value: att.FilePath})
			continue
		}
		noteLines = append(noteLines, baseNote)
	}

	if len(noteLines) > 0 {
		parts = append(parts, userContentPart{
			kind:  userPartNoteLine,
			value: "Attached files are available in this session. Use artifact_reader to inspect them:\n" + strings.Join(noteLines, "\n"),
		})
	}
	if len(parts) == 0 {
		parts = append(parts, userContentPart{kind: userPartText, value: msg.Content()})
	}
	return parts
}

// openAIPartsFromUserContent converts normalized user content parts to OpenAI message content list.
func openAIPartsFromUserContent(parts []userContentPart) responses.ResponseInputMessageContentListParam {
	out := make(responses.ResponseInputMessageContentListParam, 0, len(parts))
	for _, p := range parts {
		switch p.kind {
		case userPartText:
			out = append(out, responses.ResponseInputContentParamOfInputText(p.value))
		case userPartImageFileID:
			out = append(out, responses.ResponseInputContentUnionParam{
				OfInputImage: &responses.ResponseInputImageParam{
					FileID: openai.String(p.value),
					Detail: responses.ResponseInputImageDetailLow,
				},
			})
		case userPartImageURL:
			out = append(out, responses.ResponseInputContentUnionParam{
				OfInputImage: &responses.ResponseInputImageParam{
					ImageURL: openai.String(p.value),
					Detail:   responses.ResponseInputImageDetailLow,
				},
			})
		case userPartNoteLine:
			out = append(out, responses.ResponseInputContentParamOfInputText(p.value))
		}
	}
	return out
}

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
	resolved := m.resolveImageUploadsForMessages(ctx, messages)
	validToolCallIDs := m.validToolCallIDsFromMessages(messages)

	items := make(responses.ResponseInputParam, 0, len(messages))

	for _, msg := range messages {
		switch msg.Role() {
		case types.RoleSystem:
			items = append(items, responses.ResponseInputItemParamOfMessage(
				msg.Content(),
				responses.EasyInputMessageRoleDeveloper,
			))

		case types.RoleUser:
			parts := normalizeUserContent(msg, resolved)
			if len(parts) == 0 {
				items = append(items, responses.ResponseInputItemParamOfMessage(
					"",
					responses.EasyInputMessageRoleUser,
				))
			} else if len(parts) == 1 && parts[0].kind == userPartText {
				items = append(items, responses.ResponseInputItemParamOfMessage(
					parts[0].value,
					responses.EasyInputMessageRoleUser,
				))
			} else {
				items = append(items, responses.ResponseInputItemParamOfMessage(
					openAIPartsFromUserContent(parts),
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
					continue
				}
				items = append(items, responses.ResponseInputItemParamOfFunctionCall(
					tc.Arguments,
					callID,
					callName,
				))
			}

		case types.RoleTool:
			if msg.ToolCallID() == nil {
				continue
			}
			callID := strings.TrimSpace(*msg.ToolCallID())
			if callID == "" {
				continue
			}
			if _, ok := validToolCallIDs[callID]; !ok {
				continue
			}
			items = append(items, responses.ResponseInputItemParamOfFunctionCallOutput(
				callID,
				msg.Content(),
			))
		}
	}

	return items
}
