// Package persistence provides this package.
package persistence

import (
	"encoding/json"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

type messageScanner interface {
	Scan(dest ...any) error
}

func scanMessageModel(scanner messageScanner) (models.MessageModel, error) {
	var model models.MessageModel
	err := scanner.Scan(
		&model.ID,
		&model.SessionID,
		&model.Role,
		&model.Content,
		&model.AuthorUserID,
		&model.AuthorFirstName,
		&model.AuthorLastName,
		&model.ToolCallsJSON,
		&model.ToolCallID,
		&model.CitationsJSON,
		&model.DebugTraceJSON,
		&model.QuestionDataJSON,
		&model.CreatedAt,
	)
	if err != nil {
		return models.MessageModel{}, err
	}

	return model, nil
}

func messageDomainToModel(msg types.Message) (*models.MessageModel, error) {
	if msg == nil {
		return nil, models.ErrNilMessage
	}

	toolCallsJSON, err := json.Marshal(msg.ToolCalls())
	if err != nil {
		return nil, err
	}

	citationsJSON, err := json.Marshal(msg.Citations())
	if err != nil {
		return nil, err
	}

	debugTraceJSON, err := json.Marshal(msg.DebugTrace())
	if err != nil {
		return nil, err
	}

	questionDataJSON, err := json.Marshal(msg.QuestionData())
	if err != nil {
		return nil, err
	}

	return &models.MessageModel{
		ID:               msg.ID(),
		SessionID:        msg.SessionID(),
		Role:             msg.Role(),
		Content:          msg.Content(),
		AuthorUserID:     msg.AuthorUserID(),
		AuthorFirstName:  msg.AuthorFirstName(),
		AuthorLastName:   msg.AuthorLastName(),
		ToolCallsJSON:    toolCallsJSON,
		ToolCallID:       msg.ToolCallID(),
		CitationsJSON:    citationsJSON,
		DebugTraceJSON:   debugTraceJSON,
		QuestionDataJSON: questionDataJSON,
		CreatedAt:        msg.CreatedAt(),
	}, nil
}

func messageModelToDomain(
	model *models.MessageModel,
	codeOutputs []types.CodeInterpreterOutput,
	attachments []types.Attachment,
) (types.Message, error) {
	if model == nil {
		return nil, models.ErrNilMessageModel
	}

	var toolCalls []types.ToolCall
	if err := json.Unmarshal(model.ToolCallsJSON, &toolCalls); err != nil {
		return nil, err
	}

	var citations []types.Citation
	if err := json.Unmarshal(model.CitationsJSON, &citations); err != nil {
		return nil, err
	}

	var debugTrace *types.DebugTrace
	if len(model.DebugTraceJSON) > 0 && string(model.DebugTraceJSON) != "null" {
		var trace types.DebugTrace
		if err := json.Unmarshal(model.DebugTraceJSON, &trace); err != nil {
			return nil, err
		}
		debugTrace = &trace
	}

	var questionData *types.QuestionData
	if len(model.QuestionDataJSON) > 0 && string(model.QuestionDataJSON) != "null" {
		var qd types.QuestionData
		if err := json.Unmarshal(model.QuestionDataJSON, &qd); err != nil {
			return nil, err
		}
		questionData = &qd
	}

	opts := []types.MessageOption{
		types.WithMessageID(model.ID),
		types.WithSessionID(model.SessionID),
		types.WithRole(model.Role),
		types.WithContent(model.Content),
		types.WithCreatedAt(model.CreatedAt),
	}
	if len(toolCalls) > 0 {
		opts = append(opts, types.WithToolCalls(toolCalls...))
	}
	if model.ToolCallID != nil {
		opts = append(opts, types.WithToolCallID(*model.ToolCallID))
	}
	if len(citations) > 0 {
		opts = append(opts, types.WithCitations(citations...))
	}
	if len(codeOutputs) > 0 {
		opts = append(opts, types.WithCodeOutputs(codeOutputs...))
	}
	if len(attachments) > 0 {
		opts = append(opts, types.WithAttachments(attachments...))
	}
	if debugTrace != nil {
		opts = append(opts, types.WithDebugTrace(debugTrace))
	}
	if questionData != nil {
		opts = append(opts, types.WithQuestionData(questionData))
	}
	if model.AuthorUserID != nil {
		opts = append(opts, types.WithAuthorUserID(*model.AuthorUserID))
	}
	if strings.TrimSpace(model.AuthorFirstName) != "" || strings.TrimSpace(model.AuthorLastName) != "" {
		opts = append(opts, types.WithAuthorName(model.AuthorFirstName, model.AuthorLastName))
	}

	return types.NewMessage(opts...), nil
}
