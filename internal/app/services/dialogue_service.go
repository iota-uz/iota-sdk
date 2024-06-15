package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/iota-agency/iota-erp/internal/app/services/chatfuncs"
	"github.com/iota-agency/iota-erp/internal/domain/dialogue"
	localComposables "github.com/iota-agency/iota-erp/pkg/composables"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/iota-agency/iota-erp/sdk/utils/env"
	"github.com/sashabaranov/go-openai"
	"io"
)

type DialogueService struct {
	repo dialogue.Repository
	app  *Application
}

var (
	ErrMessageTooLong = errors.New("message is too long")
	ErrModelRequired  = errors.New("model is required")
)

func NewDialogueService(repo dialogue.Repository, app *Application) *DialogueService {
	return &DialogueService{
		repo: repo,
		app:  app,
	}
}

func (s *DialogueService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *DialogueService) GetAll(ctx context.Context) ([]*dialogue.Dialogue, error) {
	return s.repo.GetAll(ctx)
}

func (s *DialogueService) GetUserDialogues(ctx context.Context, userID int64) ([]*dialogue.Dialogue, error) {
	return s.repo.GetByUserID(ctx, userID)
}

func (s *DialogueService) GetByID(ctx context.Context, id int64) (*dialogue.Dialogue, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *DialogueService) GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*dialogue.Dialogue, error) {
	return s.repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *DialogueService) streamCompletions(ctx context.Context, model string) (*openai.ChatCompletionMessage, error) {
	client := openai.NewClient(env.MustGetEnv("OPENAI_API"))
	stream, err := client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{{
			Role: openai.ChatMessageRoleSystem,
		}},
		Stream: true,
	})
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	defer func() {
		s.app.EventPublisher.Publish("completions.end", true)
	}()
	response := &openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleAssistant,
		Content: "",
	}
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		delta := chunk.Choices[0].Delta
		if delta.Content != "" {
			response.Content += delta.Content
			s.app.EventPublisher.Publish("completions.delta", delta)
		}
		if delta.ToolCalls == nil {
			continue
		}
		for _, call := range delta.ToolCalls {
			index := *call.Index
			if len(response.ToolCalls) <= index {
				response.ToolCalls = append(response.ToolCalls, call)
				s.app.EventPublisher.Publish("completions.tool_call", call)
				continue
			}
			current := response.ToolCalls[index]
			args := current.Function.Arguments
			if call.Function.Arguments != "" {
				args += call.Function.Arguments
			}
			funcName := current.Function.Name
			if call.Function.Name != "" {
				funcName = call.Function.Name
			}
			response.ToolCalls[index] = openai.ToolCall{
				Index:    current.Index,
				ID:       current.ID,
				Type:     current.Type,
				Function: openai.FunctionCall{Name: funcName, Arguments: args},
			}
		}
	}
	return response, nil
}

func (s *DialogueService) StartDialogue(ctx context.Context, message string, model string) (*dialogue.Dialogue, error) {
	if !composables.UseAuthenticated(ctx) {
		return nil, errors.New("authentication required")
	}
	if len(message) > 1000 {
		return nil, ErrMessageTooLong
	}
	if model == "" {
		return nil, ErrModelRequired
	}
	prompt, err := s.app.PromptService.GetByID(ctx, "bi-chat")
	if err != nil {
		return nil, err
	}
	u, ok := localComposables.UseUser(ctx)
	if !ok {
		return nil, fmt.Errorf("user not found")
	}

	data := &dialogue.Dialogue{
		UserID: u.Id,
		Messages: dialogue.Messages{
			{Role: openai.ChatMessageRoleSystem, Content: prompt.Prompt},
			{Role: openai.ChatMessageRoleUser, Content: message},
		},
		Label: "Новый чат",
	}
	if err := s.repo.Create(ctx, data); err != nil {
		return nil, err
	}
	s.app.EventPublisher.Publish("dialogue.created", data)
	s.app.EventPublisher.Publish("completions.start", true)
	db, ok := composables.UseTx(ctx)
	if !ok {
		return nil, errors.New("transaction not found")
	}
	tools, err := chatfuncs.GetTools(db)
	if err != nil {
		return nil, err
	}
	for i := 0; i < 10; i++ {
		msg, err := s.streamCompletions(ctx, model)
		if err != nil {
			return nil, err
		}
		data.AddMessage(*msg)
		if err := s.repo.Update(ctx, data); err != nil {
			return nil, err
		}
		s.app.EventPublisher.Publish("dialogue.updated", data)
		for _, call := range msg.ToolCalls {
			funcName := call.Function.Name
			//if funcName == "do_sql_query" && !tools.HasCalledMethod("get_schema") {
			//	messages = append(messages, openai.ChatCompletionMessage{
			//		Role:    openai.ChatMessageRoleTool,
			//		Content: `{"error": "You must call 'get_schema' first."}`,
			//	})
			//	break
			//}
			if fn, ok := tools.Funcs[funcName]; ok {
				args := call.Function.Arguments
				parsedArgs := map[string]interface{}{}
				if err := json.Unmarshal([]byte(args), &parsedArgs); err != nil {
					return nil, err
				}
				result, err := fn(parsedArgs)
				if err != nil {
					return nil, err
				}
				data.AddMessage(openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleTool,
					Content: result,
				})
			}
		}
	}
	return data, nil
}

func (s *DialogueService) Update(ctx context.Context, data *dialogue.Dialogue) error {
	if err := s.repo.Update(ctx, data); err != nil {
		return err
	}
	s.app.EventPublisher.Publish("dialogue.updated", data)
	return nil
}

func (s *DialogueService) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.app.EventPublisher.Publish("dialogue.deleted", id)
	return nil
}
