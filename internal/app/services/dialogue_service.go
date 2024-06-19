package services

import (
	"context"
	"errors"
	"fmt"
	"github.com/iota-agency/iota-erp/internal/configuration"
	"github.com/iota-agency/iota-erp/internal/domain/dialogue"
	localComposables "github.com/iota-agency/iota-erp/pkg/composables"
	"github.com/iota-agency/iota-erp/sdk/llm/gpt-functions"
	"github.com/sashabaranov/go-openai"
	"io"
	"log"
	"time"
)

type DialogueService struct {
	repo      dialogue.Repository
	app       *Application
	chatFuncs *functions.ChatTools
}

var (
	ErrMessageTooLong = errors.New("message is too long")
	ErrModelRequired  = errors.New("model is required")
)

func NewDialogueService(repo dialogue.Repository, app *Application) *DialogueService {
	chatFuncs := functions.New()
	//chatFuncs.Add(functions.NewGetSchema(app.Db))
	//chatFuncs.Add(chatfuncs.NewCurrencyConvert())
	//chatFuncs.Add(chatfuncs.NewDoSQLQuery(app.Db))
	chatFuncs.Add(NewSearchKnowledgeBase(app.EmbeddingService))
	return &DialogueService{
		repo:      repo,
		app:       app,
		chatFuncs: chatFuncs,
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

func (s *DialogueService) streamCompletions(
	ctx context.Context,
	messages []openai.ChatCompletionMessage,
	model string,
) (chan openai.ChatCompletionMessage, error) {
	client := openai.NewClient(configuration.Use().OpenAIKey())
	stream, err := client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Tools:    s.chatFuncs.OpenAiTools(),
		Stream:   true,
	})
	if err != nil {
		return nil, err
	}
	ch := make(chan openai.ChatCompletionMessage)
	response := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleAssistant,
		Content: "",
	}
	go func() {
		defer stream.Close()
		for {
			chunk, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				log.Println(err)
				break
			}
			delta := chunk.Choices[0].Delta
			if delta.Content != "" {
				response.Content += delta.Content
				ch <- response
			}
			if delta.ToolCalls == nil {
				continue
			}
			for _, call := range delta.ToolCalls {
				index := *call.Index
				if len(response.ToolCalls) <= index {
					response.ToolCalls = append(response.ToolCalls, call)
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
				ch <- response
			}
		}
		close(ch)
	}()
	return ch, nil
}

func (s *DialogueService) fakeStream() (chan openai.ChatCompletionMessage, error) {
	ch := make(chan openai.ChatCompletionMessage)
	msg := "Hello, how can I help you?"
	go func() {
		for i, _ := range msg {
			ch <- openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: msg[:i+1],
			}
			time.Sleep(40 * time.Millisecond)
		}
	}()
	return ch, nil
}

func (s *DialogueService) ChatComplete(ctx context.Context, data *dialogue.Dialogue, model string) error {
	for i := 0; i < 10; i++ {
		ch, err := s.streamCompletions(ctx, data.Messages, model)
		if err != nil {
			return err
		}
		data.AddMessage(openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: "",
		})
		for m := range ch {
			data.Messages[len(data.Messages)-1] = m
			s.app.EventPublisher.Publish("dialogue.updated", data)
		}
		msg := data.Messages[len(data.Messages)-1]
		if err := s.repo.Update(ctx, data); err != nil {
			return err
		}
		if len(msg.ToolCalls) == 0 {
			break
		}
		for _, call := range msg.ToolCalls {
			funcName := call.Function.Name
			//if funcName == "do_sql_query" && !tools.HasCalledMethod("get_schema") {
			//	messages = append(messages, openai.ChatCompletionMessage{
			//		Role:    openai.ChatMessageRoleTool,
			//		Content: `{"error": "You must call 'get_schema' first."}`,
			//	})
			//	break
			//}

			result, err := s.chatFuncs.Call(funcName, call.Function.Arguments)
			if err != nil {
				return err
			}
			data.AddMessage(openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				ToolCallID: call.ID,
				Content:    result,
			})
		}
	}
	return nil
}

func (s *DialogueService) ReplyToDialogue(ctx context.Context, dialogueId int64, message string, model string) (*dialogue.Dialogue, error) {
	if len(message) > 1000 {
		return nil, ErrMessageTooLong
	}
	if model == "" {
		return nil, ErrModelRequired
	}
	data, err := s.GetByID(ctx, dialogueId)
	if err != nil {
		return nil, err
	}
	data.AddMessage(openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: message,
	})
	if err := s.repo.Update(ctx, data); err != nil {
		return nil, err
	}
	if err := s.ChatComplete(ctx, data, model); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *DialogueService) StartDialogue(ctx context.Context, message string, model string) (*dialogue.Dialogue, error) {
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
	if err := s.ChatComplete(ctx, data, model); err != nil {
		return nil, err
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
