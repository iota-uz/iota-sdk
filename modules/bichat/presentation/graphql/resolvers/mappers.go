package resolvers

import (
	"github.com/iota-uz/iota-sdk/modules/bichat/presentation/graphql/model"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// toGraphQLSession converts a domain Session to a GraphQL Session
func toGraphQLSession(s *domain.Session) *model.Session {
	if s == nil {
		return nil
	}

	gqlSession := &model.Session{
		ID:        s.ID.String(),
		TenantID:  s.TenantID.String(),
		UserID:    int(s.UserID),
		Title:     s.Title,
		Status:    toGraphQLSessionStatus(s.Status),
		Pinned:    s.Pinned,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
		Messages:  []*model.Message{}, // Messages loaded separately
	}

	if s.ParentSessionID != nil {
		parentID := s.ParentSessionID.String()
		gqlSession.ParentSessionID = &parentID
	}

	if s.PendingQuestionAgent != nil {
		gqlSession.PendingQuestionAgent = s.PendingQuestionAgent
	}

	return gqlSession
}

// toGraphQLSessionStatus converts domain SessionStatus to GraphQL SessionStatus
func toGraphQLSessionStatus(status domain.SessionStatus) model.SessionStatus {
	switch status {
	case domain.SessionStatusActive:
		return model.SessionStatusActive
	case domain.SessionStatusArchived:
		return model.SessionStatusArchived
	default:
		return model.SessionStatusActive
	}
}

// toGraphQLMessage converts a types.Message to a GraphQL Message
func toGraphQLMessage(m *types.Message) *model.Message {
	if m == nil {
		return nil
	}

	gqlMessage := &model.Message{
		ID:          m.ID.String(),
		SessionID:   m.SessionID.String(),
		Role:        toGraphQLMessageRole(m.Role),
		Content:     m.Content,
		ToolCalls:   toGraphQLToolCalls(m.ToolCalls),
		Attachments: toGraphQLAttachments(m.Attachments),
		Citations:   toGraphQLCitations(m.Citations),
		CodeOutputs: toGraphQLCodeOutputs(m.CodeOutputs),
		CreatedAt:   m.CreatedAt,
	}

	if m.ToolCallID != nil {
		gqlMessage.ToolCallID = m.ToolCallID
	}

	return gqlMessage
}

// toGraphQLCodeOutputs converts []types.CodeInterpreterOutput to []*model.CodeInterpreterOutput
func toGraphQLCodeOutputs(outputs []types.CodeInterpreterOutput) []*model.CodeInterpreterOutput {
	if len(outputs) == 0 {
		return []*model.CodeInterpreterOutput{}
	}

	result := make([]*model.CodeInterpreterOutput, len(outputs))
	for i, o := range outputs {
		result[i] = &model.CodeInterpreterOutput{
			ID:        o.ID.String(),
			Name:      o.Name,
			MimeType:  o.MimeType,
			URL:       o.URL,
			Size:      o.Size,
			CreatedAt: o.CreatedAt,
		}
	}
	return result
}

// toGraphQLMessageRole converts types.Role to GraphQL MessageRole
func toGraphQLMessageRole(role types.Role) model.MessageRole {
	switch role {
	case types.RoleUser:
		return model.MessageRoleUser
	case types.RoleAssistant:
		return model.MessageRoleAssistant
	case types.RoleTool:
		return model.MessageRoleTool
	case types.RoleSystem:
		return model.MessageRoleSystem
	default:
		return model.MessageRoleUser
	}
}

// toGraphQLToolCalls converts []types.ToolCall to []*model.ToolCall
func toGraphQLToolCalls(toolCalls []types.ToolCall) []*model.ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}

	result := make([]*model.ToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		result[i] = &model.ToolCall{
			ID:        tc.ID,
			Name:      tc.Name,
			Arguments: tc.Arguments,
		}
	}
	return result
}

// toGraphQLAttachments converts []types.Attachment to []*model.Attachment
func toGraphQLAttachments(attachments []types.Attachment) []*model.Attachment {
	if len(attachments) == 0 {
		return []*model.Attachment{}
	}

	result := make([]*model.Attachment, len(attachments))
	for i, a := range attachments {
		result[i] = &model.Attachment{
			ID:          a.ID.String(),
			MessageID:   a.MessageID.String(),
			FileName:    a.FileName,
			MimeType:    a.MimeType,
			SizeBytes:   a.SizeBytes,
			StoragePath: a.FilePath,
			CreatedAt:   a.CreatedAt,
		}
	}
	return result
}

// toGraphQLCitations converts []types.Citation to []*model.Citation
func toGraphQLCitations(citations []types.Citation) []*model.Citation {
	if len(citations) == 0 {
		return []*model.Citation{}
	}

	result := make([]*model.Citation, len(citations))
	for i, c := range citations {
		citation := &model.Citation{
			Source: c.Type,
		}
		if c.Title != "" {
			citation.Title = &c.Title
		}
		if c.URL != "" {
			citation.URL = &c.URL
		}
		if c.Excerpt != "" {
			citation.Excerpt = &c.Excerpt
		}
		result[i] = citation
	}
	return result
}

// toGraphQLInterrupt converts a services.Interrupt to a GraphQL Interrupt
func toGraphQLInterrupt(interrupt *services.Interrupt) *model.Interrupt {
	if interrupt == nil {
		return nil
	}

	gqlInterrupt := &model.Interrupt{
		CheckpointID: interrupt.CheckpointID,
		Questions:    make([]*model.Question, len(interrupt.Questions)),
	}

	for i, q := range interrupt.Questions {
		gqlInterrupt.Questions[i] = toGraphQLQuestion(q)
	}

	return gqlInterrupt
}

// toGraphQLQuestion converts a services.Question to a GraphQL Question
func toGraphQLQuestion(q services.Question) *model.Question {
	gqlQuestion := &model.Question{
		ID:          q.ID,
		Question:    q.Text,
		Header:      q.Text, // Use text as header for now
		MultiSelect: q.Type == services.QuestionTypeMultipleChoice,
		Options:     make([]*model.QuestionOption, len(q.Options)),
	}

	for i, opt := range q.Options {
		gqlQuestion.Options[i] = &model.QuestionOption{
			Label:       opt.Label,
			Description: opt.Label, // Use label as description for now
		}
	}

	return gqlQuestion
}

// strPtr is a helper to convert string to *string
func strPtr(s string) *string {
	return &s
}
