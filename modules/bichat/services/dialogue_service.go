package services

import (
	"context"
	"errors"
	"io"
	"log"
	"time"

	"github.com/iota-uz/iota-sdk/modules/bichat/domain/entities/dialogue"
	"github.com/iota-uz/iota-sdk/modules/bichat/domain/entities/llm"
	"github.com/iota-uz/iota-sdk/modules/bichat/domain/entities/prompt"
	"github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/llmproviders"

	"github.com/iota-uz/iota-sdk/pkg/application"
	localComposables "github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	functions "github.com/iota-uz/iota-sdk/pkg/llm/gpt-functions"

	"github.com/sashabaranov/go-openai"
)

type DialogueService struct {
	repo           dialogue.Repository
	eventBus       eventbus.EventBus
	chatFuncs      *functions.ChatTools
	openaiProvider *llmproviders.OpenAIProvider
	//promptService  *PromptService
}

var (
	ErrMessageTooLong = errors.New("message is too long")
	ErrModelRequired  = errors.New("model is required")
)

func NewDialogueService(repo dialogue.Repository, app application.Application) *DialogueService {
	chatFuncs := functions.New()

	// chatFuncs.Add(chatfuncs.NewCurrencyConvert())
	// chatFuncs.Add(chatfuncs.NewDoSQLQuery(app.DB))
	chatFuncs.Add(NewSearchKnowledgeBase(app.Service(EmbeddingService{}).(*EmbeddingService)))
	return &DialogueService{
		repo:      repo,
		eventBus:  app.EventPublisher(),
		chatFuncs: chatFuncs,
		//promptService:  app.Service(PromptService{}).(*PromptService),
		openaiProvider: llmproviders.NewOpenAIProvider(configuration.Use().OpenAIKey),
	}
}

func (s *DialogueService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *DialogueService) GetAll(ctx context.Context) ([]dialogue.Dialogue, error) {
	return s.repo.GetAll(ctx)
}

func (s *DialogueService) GetUserDialogues(ctx context.Context, userID uint) ([]dialogue.Dialogue, error) {
	return s.repo.GetByUserID(ctx, userID)
}

func (s *DialogueService) GetByID(ctx context.Context, id uint) (dialogue.Dialogue, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *DialogueService) GetPaginated(
	ctx context.Context,
	params *dialogue.FindParams,
) ([]dialogue.Dialogue, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *DialogueService) streamCompletions(
	ctx context.Context,
	messages []llm.ChatCompletionMessage,
	model string,
) (chan openai.ChatCompletionMessage, error) {
	stream, err := s.openaiProvider.CreateChatCompletionStream(ctx, llm.ChatCompletionRequest{
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
		defer func(stream *openai.ChatCompletionStream) {
			if err := stream.Close(); err != nil {
				log.Println(err)
			}
		}(stream)
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
		for i := range msg {
			ch <- openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: msg[:i+1],
			}
			time.Sleep(40 * time.Millisecond)
		}
	}()
	return ch, nil
}

func (s *DialogueService) ChatComplete(ctx context.Context, data dialogue.Dialogue, model string) error {
	for range 10 {
		//ch, err := s.streamCompletions(ctx, data.Messages(), model)
		ch, err := s.fakeStream()
		if err != nil {
			return err
		}
		data = data.AddMessages(llm.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: "",
		})
		for m := range ch {
			data.SetLastMessage(llmproviders.OpenAIChatCompletionMessageToDomain(m))
			s.eventBus.Publish(dialogue.UpdatedEvent{
				Result: data,
			})
		}
		if err := s.repo.Update(ctx, data); err != nil {
			return err
		}
		msg := data.LastMessage()
		if len(msg.ToolCalls) == 0 {
			break
		}
		for _, call := range msg.ToolCalls {
			funcName := call.Function.Name
			// if funcName == "do_sql_query" && !tools.HasCalledMethod("get_schema") {
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
			data.AddMessages(llm.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				ToolCallID: call.ID,
				Content:    result,
			})
		}
	}
	return nil
}

func (s *DialogueService) ReplyToDialogue(
	ctx context.Context,
	dialogueID uint,
	message, model string,
) (dialogue.Dialogue, error) {
	if len(message) > 1000 {
		return nil, ErrMessageTooLong
	}
	if model == "" {
		return nil, ErrModelRequired
	}
	data, err := s.GetByID(ctx, dialogueID)
	if err != nil {
		return nil, err
	}
	data = data.AddMessages(llm.ChatCompletionMessage{
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

func (s *DialogueService) StartDialogue(ctx context.Context, message string, model string) (dialogue.Dialogue, error) {
	if len(message) > 1000 {
		return nil, ErrMessageTooLong
	}
	if model == "" {
		return nil, ErrModelRequired
	}
	p := prompt.Prompt{
		Prompt: "YOU ARE A HELP FULL ASSISTANT FOR AN ERP USER",
	}
	u, err := localComposables.UseUser(ctx)
	if err != nil {
		return nil, err
	}
	tenant, err := localComposables.UseTenant(ctx)
	if err != nil {
		return nil, err
	}
	data := dialogue.New(
		tenant.ID,
		u.ID(),
		"Новый чат",
	).AddMessages(
		llm.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: p.Prompt,
		},
		llm.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: message,
		},
	)
	result, err := s.repo.Create(ctx, data)
	if err != nil {
		return nil, err
	}
	createdEvent, err := dialogue.NewCreatedEvent(ctx, data, result)
	if err != nil {
		return nil, err
	}
	s.eventBus.Publish(createdEvent)
	if err := s.ChatComplete(ctx, data, model); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *DialogueService) Update(ctx context.Context, data dialogue.Dialogue) error {
	if err := s.repo.Update(ctx, data); err != nil {
		return err
	}
	updatedEvent, err := dialogue.NewUpdatedEvent(ctx, data, data)
	if err != nil {
		return err
	}
	s.eventBus.Publish(updatedEvent)
	return nil
}

func (s *DialogueService) Delete(ctx context.Context, id uint) (dialogue.Dialogue, error) {
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	deletedEvent, err := dialogue.NewDeletedEvent(ctx, entity)
	if err != nil {
		return nil, err
	}
	s.eventBus.Publish(deletedEvent)
	return entity, nil
}
