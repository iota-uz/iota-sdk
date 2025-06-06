package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/website/domain/entities/aichatconfig"
	"github.com/iota-uz/iota-sdk/modules/website/domain/entities/chatthread"
	"github.com/iota-uz/iota-sdk/modules/website/infrastructure/rag"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/sirupsen/logrus"
)

var thinkTagRegex = regexp.MustCompile(`(?s)<think>.*?</think>`)

type CreateThreadDTO struct {
	Phone   string
	Country country.Country
}

type SendMessageToThreadDTO struct {
	ThreadID uuid.UUID
	Message  string
}

type ReplyToThreadDTO struct {
	ThreadID uuid.UUID
	UserID   uint
	Message  string
}

type WebsiteChatServiceConfig struct {
	AIConfigRepo aichatconfig.Repository
	UserRepo     user.Repository
	ClientRepo   client.Repository
	ChatRepo     chat.Repository
	AIUserEmail  internet.Email
	RAGProvider  rag.Provider
}

type WebsiteChatService struct {
	aiconfigRepo aichatconfig.Repository
	userRepo     user.Repository
	clientRepo   client.Repository
	chatRepo     chat.Repository
	aiUserEmail  internet.Email
	ragProvider  rag.Provider
	threadsMap   ThreadsMap
}

type ThreadsMap map[uuid.UUID]chatthread.ChatThread

func NewWebsiteChatService(config WebsiteChatServiceConfig) *WebsiteChatService {
	return &WebsiteChatService{
		aiconfigRepo: config.AIConfigRepo,
		userRepo:     config.UserRepo,
		clientRepo:   config.ClientRepo,
		chatRepo:     config.ChatRepo,
		aiUserEmail:  config.AIUserEmail,
		ragProvider:  config.RAGProvider,
		threadsMap:   make(ThreadsMap),
	}
}

func (s *WebsiteChatService) GetThreadByID(ctx context.Context, threadID uuid.UUID) (chatthread.ChatThread, error) {
	thread, ok := s.threadsMap[threadID]
	if !ok {
		return nil, chatthread.ErrChatThreadNotFound
	}
	chatEntity, err := s.chatRepo.GetByID(ctx, thread.ChatID())
	if err != nil {
		return nil, err
	}
	return chatthread.New(
		chatEntity.ID(),
		chatEntity.Messages(),
		chatthread.WithID(thread.ID()),
		chatthread.WithTimestamp(thread.Timestamp()),
	), nil
}

func (s *WebsiteChatService) CreateThread(ctx context.Context, dto CreateThreadDTO) (chatthread.ChatThread, error) {
	var member chat.Member
	p, err := phone.Parse(dto.Phone, dto.Country)
	if err != nil {
		return nil, err
	}
	member, err = s.memberFromPhone(ctx, p)
	if err != nil {
		return nil, err
	}
	tenant, err := composables.UseTenant(ctx)
	if err != nil {
		return nil, err
	}

	chatEntity := chat.New(
		member.Sender().(chat.ClientSender).ClientID(),
		chat.WithMembers([]chat.Member{member}),
		chat.WithTenantID(tenant.ID),
	)

	var createdChat chat.Chat
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		v, e := s.chatRepo.Save(txCtx, chatEntity)
		if e != nil {
			return e
		}
		createdChat = v
		return nil
	})
	if err != nil {
		return nil, err
	}

	threadID := uuid.New()
	thread := chatthread.New(
		createdChat.ID(),
		createdChat.Messages(),
		chatthread.WithID(threadID),
	)
	s.threadsMap[threadID] = thread

	return thread, nil
}

func (s *WebsiteChatService) SendMessageToThread(
	ctx context.Context,
	dto SendMessageToThreadDTO,
) (chatthread.ChatThread, error) {
	if dto.Message == "" {
		return nil, chat.ErrEmptyMessage
	}
	thread, err := s.GetThreadByID(ctx, dto.ThreadID)
	if err != nil {
		return nil, err
	}

	chatEntity, err := s.chatRepo.GetByID(ctx, thread.ChatID())
	if err != nil {
		return nil, err
	}
	var member chat.Member
	for _, m := range chatEntity.Members() {
		if m.Transport() != chat.WebsiteTransport {
			continue
		}
		if v, ok := m.Sender().(chat.ClientSender); ok && v.ClientID() == chatEntity.ClientID() {
			member = m
			break
		}
	}

	if member == nil {
		return nil, fmt.Errorf("no client member found in chat")
	}

	t := time.Now()
	chatEntity = chatEntity.AddMessage(chat.NewMessage(
		dto.Message,
		member,
		chat.WithMessageSentAt(&t),
		chat.WithMessageCreatedAt(t),
	))
	if err != nil {
		return nil, err
	}

	var savedChat chat.Chat
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		v, e := s.chatRepo.Save(txCtx, chatEntity)
		if e != nil {
			return e
		}
		savedChat = v
		return nil
	})
	if err != nil {
		return nil, err
	}

	updatedThread := chatthread.New(
		savedChat.ID(),
		savedChat.Messages(),
		chatthread.WithID(thread.ID()),
		chatthread.WithTimestamp(thread.Timestamp()),
	)
	s.threadsMap[thread.ID()] = updatedThread

	return updatedThread, nil
}

func (s *WebsiteChatService) ReplyToThread(
	ctx context.Context,
	dto ReplyToThreadDTO,
) (chatthread.ChatThread, error) {
	thread, err := s.GetThreadByID(ctx, dto.ThreadID)
	if err != nil {
		return nil, err
	}

	if dto.Message == "" {
		return nil, chat.ErrEmptyMessage
	}

	var member chat.Member

	chatEntity, err := s.chatRepo.GetByID(ctx, thread.ChatID())
	if err != nil {
		return nil, err
	}
	for _, m := range chatEntity.Members() {
		if m.Transport() != chat.WebsiteTransport {
			continue
		}

		if v, ok := m.Sender().(chat.UserSender); ok && v.UserID() == dto.UserID {
			member = m
			break
		}
	}

	if member == nil {
		member, err = s.memberFromUserID(ctx, dto.UserID)
		if err != nil {
			return nil, err
		}
	}

	t := time.Now()
	chatEntity = chatEntity.AddMessage(chat.NewMessage(
		dto.Message,
		member,
		chat.WithMessageSentAt(&t),
		chat.WithMessageCreatedAt(t),
	))
	if err != nil {
		return nil, err
	}

	var savedChat chat.Chat
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		v, e := s.chatRepo.Save(txCtx, chatEntity)
		if e != nil {
			return e
		}
		savedChat = v
		return nil
	})
	if err != nil {
		return nil, err
	}

	updatedThread := chatthread.New(
		savedChat.ID(),
		savedChat.Messages(),
		chatthread.WithID(thread.ID()),
		chatthread.WithTimestamp(thread.Timestamp()),
	)
	s.threadsMap[thread.ID()] = updatedThread

	return updatedThread, nil
}

func (s *WebsiteChatService) GetAvailableModels(ctx context.Context) ([]string, error) {
	config, err := s.aiconfigRepo.GetDefault(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get AI configuration: %w", err)
	}

	return s.getModelsWithConfig(ctx, config.BaseURL(), config.AccessToken())
}

func (s *WebsiteChatService) GetAvailableModelsWithConfig(ctx context.Context, baseURL, accessToken string) ([]string, error) {
	if baseURL == "" || accessToken == "" {
		return nil, fmt.Errorf("baseURL and accessToken are required")
	}
	return s.getModelsWithConfig(ctx, baseURL, accessToken)
}

func (s *WebsiteChatService) getModelsWithConfig(ctx context.Context, baseURL, accessToken string) ([]string, error) {
	var openaiClient openai.Client
	if baseURL != "" {
		openaiClient = openai.NewClient(
			option.WithAPIKey(accessToken),
			option.WithBaseURL(baseURL),
		)
	} else {
		openaiClient = openai.NewClient(
			option.WithAPIKey(accessToken),
		)
	}

	response, err := openaiClient.Models.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}

	models := make([]string, 0, len(response.Data))
	for _, model := range response.Data {
		models = append(models, model.ID)
	}

	return models, nil
}

func (s *WebsiteChatService) ReplyWithAI(ctx context.Context, threadID uuid.UUID) (chatthread.ChatThread, error) {
	logger := composables.UseLogger(ctx)
	thread, err := s.GetThreadByID(ctx, threadID)
	if err != nil {
		return nil, err
	}

	messages := thread.Messages()
	if len(messages) == 0 {
		return nil, chat.ErrNoMessages
	}

	openaiMessages := []openai.ChatCompletionMessageParamUnion{}

	config, err := s.aiconfigRepo.GetDefault(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get AI configuration: %w", err)
	}

	if config.SystemPrompt() != "" {
		tmpl, err := template.New("system_prompt").Parse(config.SystemPrompt())
		if err != nil {
			return nil, fmt.Errorf("failed to parse system prompt template: %w", err)
		}

		var buf bytes.Buffer
		templateData := map[string]interface{}{}
		err = tmpl.Execute(&buf, templateData)
		if err != nil {
			return nil, fmt.Errorf("failed to execute system prompt template: %w", err)
		}

		openaiMessages = append(openaiMessages, openai.SystemMessage(buf.String()))
	}

	if s.ragProvider != nil && len(messages) > 0 {
		lastMessage := messages[len(messages)-1]
		chunks, err := s.ragProvider.SearchRelevantContext(ctx, lastMessage.Message())
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve context: %w", err)
		}
		if len(chunks) > 0 {
			docsText := strings.Join(chunks, "\n---\n")
			logger.WithFields(logrus.Fields{
				"thread_id":    threadID,
				"chunks":       len(chunks),
				"context":      docsText,
				"user_message": lastMessage.Message(),
			}).Info("Retrieved context for AI response")
			openaiMessages = append(openaiMessages, openai.AssistantMessage("Retrieved context:\n"+docsText))
		}
	}

	for _, msg := range messages {
		if msg.Sender().Sender().Type() == chat.UserSenderType {
			openaiMessages = append(openaiMessages, openai.AssistantMessage(msg.Message()))
		} else {
			openaiMessages = append(openaiMessages, openai.UserMessage(msg.Message()))
		}
	}

	var openaiClient openai.Client
	if config.BaseURL() != "" {
		openaiClient = openai.NewClient(
			option.WithAPIKey(config.AccessToken()),
			option.WithBaseURL(config.BaseURL()),
		)
	} else {
		openaiClient = openai.NewClient(
			option.WithAPIKey(config.AccessToken()),
		)
	}

	maxTokens := int64(config.MaxTokens())
	temperature := float64(config.Temperature())
	response, err := openaiClient.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:       config.ModelName(),
		Messages:    openaiMessages,
		Temperature: openai.Float(temperature),
		MaxTokens:   openai.Int(maxTokens),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get AI response: %w", err)
	}

	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("no response from AI")
	}

	rawAIResponse := response.Choices[0].Message.Content

	logger.WithFields(logrus.Fields{
		"thread_id":       threadID,
		"raw_ai_response": rawAIResponse,
	}).Info("Complete AI model output received")

	aiResponse := strings.TrimSpace(thinkTagRegex.ReplaceAllString(rawAIResponse, ""))

	aiUser, err := s.userRepo.GetByEmail(ctx, s.aiUserEmail.Value())
	if err != nil {
		return nil, fmt.Errorf("failed to get AI user: %w", err)
	}

	respThread, err := s.ReplyToThread(ctx, ReplyToThreadDTO{
		ThreadID: threadID,
		UserID:   aiUser.ID(),
		Message:  aiResponse,
	})
	if err != nil {
		return nil, err
	}

	return respThread, nil
}

func (s *WebsiteChatService) memberFromUserID(ctx context.Context, userID uint) (chat.Member, error) {
	usr, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	tenant, err := composables.UseTenant(ctx)
	if err != nil {
		return nil, err
	}

	return chat.NewMember(
		chat.NewUserSender(
			usr.ID(),
			usr.FirstName(),
			usr.LastName(),
		),
		chat.WebsiteTransport,
		chat.WithMemberTenantID(tenant.ID),
	), nil
}

func (s *WebsiteChatService) memberFromPhone(ctx context.Context, phoneNumber phone.Phone) (chat.Member, error) {
	tenant, err := composables.UseTenant(ctx)
	if err != nil {
		return nil, err
	}
	match, err := s.clientRepo.GetByContactValue(ctx, client.ContactTypePhone, phoneNumber.Value())
	if err == nil {
		var contactID uint
		for _, contact := range match.Contacts() {
			if contact.Type() == client.ContactTypePhone && contact.Value() == phoneNumber.Value() {
				contactID = contact.ID()
				break
			}
		}
		return chat.NewMember(
			chat.NewClientSender(
				match.ID(),
				contactID,
				match.FirstName(),
				match.LastName(),
			),
			chat.WebsiteTransport,
			chat.WithMemberTenantID(tenant.ID),
		), nil
	} else if err != nil && !errors.Is(err, persistence.ErrClientNotFound) {
		return nil, err
	}

	c, err := client.New(
		phoneNumber.Value(),
		client.WithPhone(phoneNumber),
		client.WithTenantID(tenant.ID),
	)
	if err != nil {
		return nil, err
	}

	var clientEntity client.Client
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		v, e := s.clientRepo.Save(txCtx, c)
		if e != nil {
			return e
		}
		clientEntity = v
		return nil
	})
	if err != nil {
		return nil, err
	}
	var contactID uint
	for _, contact := range clientEntity.Contacts() {
		if contact.Type() == client.ContactTypePhone && contact.Value() == phoneNumber.Value() {
			contactID = contact.ID()
			break
		}
	}
	member := chat.NewMember(
		chat.NewClientSender(
			clientEntity.ID(),
			contactID,
			clientEntity.FirstName(),
			clientEntity.LastName(),
		),
		chat.WebsiteTransport,
		chat.WithMemberTenantID(tenant.ID),
	)
	return member, nil
}
