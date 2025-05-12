package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	crmServices "github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/modules/website/domain/entities/aichatconfig"
	"github.com/iota-uz/iota-sdk/modules/website/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/website/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/website/presentation/templates/pages/aichat"
	"github.com/iota-uz/iota-sdk/modules/website/presentation/viewmodels"
	websiteServices "github.com/iota-uz/iota-sdk/modules/website/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type AIChatControllerConfig struct {
	BasePath string
	App      application.Application
}

func NewAIChatController(cfg AIChatControllerConfig) application.Controller {
	return &AIChatController{
		basePath:      "/website/ai-chat",
		app:           cfg.App,
		chatService:   cfg.App.Service(websiteServices.WebsiteChatService{}).(*websiteServices.WebsiteChatService),
		clientService: cfg.App.Service(crmServices.ClientService{}).(*crmServices.ClientService),
		configService: cfg.App.Service(websiteServices.AIChatConfigService{}).(*websiteServices.AIChatConfigService),
	}
}

type AIChatController struct {
	basePath      string
	app           application.Application
	chatService   *websiteServices.WebsiteChatService
	clientService *crmServices.ClientService
	configService *websiteServices.AIChatConfigService
}

func (c *AIChatController) Key() string {
	return "AiChatController"
}

func (c *AIChatController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideLocalizer(c.app.Bundle()),
		middleware.WithPageContext(),
		middleware.Tabs(),
		middleware.NavItems(),
	)
	router.HandleFunc("", c.configureAIChat).Methods(http.MethodGet)
	router.HandleFunc("/config", c.saveConfig).Methods(http.MethodPost)

	bareRouter := r.PathPrefix(c.basePath).Subrouter()
	bareRouter.HandleFunc("/payload", c.aiChat).Methods(http.MethodGet)
	bareRouter.HandleFunc("/test-wc", c.aiChatWC).Methods(http.MethodGet)
	bareRouter.HandleFunc("/messages", c.createThread).Methods(http.MethodPost)
	bareRouter.HandleFunc("/messages/{chat_id}", c.getThreadMessages).Methods(http.MethodGet)
	bareRouter.HandleFunc("/messages/{chat_id}", c.addMessageToThread).Methods(http.MethodPost)
}

func (c *AIChatController) configureAIChat(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())

	// Try to get the default configuration
	config, err := c.configService.GetDefault(r.Context())
	if err != nil && !errors.Is(err, aichatconfig.ErrConfigNotFound) {
		logger.WithError(err).Error("failed to get default AI chat configuration")
		http.Error(w, "Failed to get AI chat configuration", http.StatusInternalServerError)
		return
	}

	// Create props with default values if no config exists
	props := aichat.ConfigureProps{
		FormAction: c.basePath + "/config",
	}

	// If we have a config, add its values
	if err == nil {
		props.Config = mappers.AIConfigToViewModel(config)
	} else {
		// Create empty configuration
		props.Config = &viewmodels.AIConfig{
			ModelType:   string(aichatconfig.AIModelTypeOpenAI),
			Temperature: 0.7,
			MaxTokens:   1024,
			BaseURL:     "https://api.openai.com/v1",
		}
	}

	templ.Handler(aichat.Configure(props)).ServeHTTP(w, r)
}

func (c *AIChatController) saveConfig(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())

	dto, err := composables.UseForm(&dtos.AIConfigDTO{}, r)
	if err != nil {
		logger.WithError(err).Error("failed to parse form")
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	config, _ := c.configService.GetDefault(r.Context())
	if errors, ok := dto.Ok(r.Context()); !ok {
		logger.WithField("errors", errors).Error("validation failed")

		props := aichat.ConfigureProps{
			FormAction: c.basePath + "/config",
			Config: &viewmodels.AIConfig{
				ModelName:    dto.ModelName,
				ModelType:    dto.ModelType,
				SystemPrompt: dto.SystemPrompt,
				BaseURL:      dto.BaseURL,
			},
			Errors: errors,
		}

		if dto.Temperature > 0 {
			props.Config.Temperature = dto.Temperature
		}

		if dto.MaxTokens > 0 {
			props.Config.MaxTokens = dto.MaxTokens
		}

		if config != nil {
			props.Config.ID = config.ID().String()
		}

		templ.Handler(aichat.ConfigureForm(props)).ServeHTTP(w, r)
		return
	}

	configEntity, err := dto.Apply(config)
	if err != nil {
		logger.WithError(err).Error("failed to convert DTO to entity")
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	_, err = c.configService.Save(r.Context(), configEntity)
	if err != nil {
		logger.WithError(err).Error("failed to save AI chat configuration")
		http.Error(w, "Failed to save AI chat configuration", http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *AIChatController) aiChat(w http.ResponseWriter, r *http.Request) {
	title := r.URL.Query().Get("title")
	description := r.URL.Query().Get("description")
	templ.Handler(aichat.Chat(aichat.Props{
		Title:       title,
		Description: description,
		Endpoint:    c.basePath + "/message",
	})).ServeHTTP(w, r)
}

func (c *AIChatController) aiChatWC(w http.ResponseWriter, r *http.Request) {
	templ.Handler(aichat.WebComponent()).ServeHTTP(w, r)
}

func (c *AIChatController) createThread(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())

	var msg dtos.ChatMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		logger.WithError(err).Error("failed to decode request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	chatEntity, err := c.chatService.CreateThread(r.Context(), msg.Contact, country.Uzbekistan)
	if err != nil {
		logger.WithError(err).Error("failed to create chat thread")
		http.Error(w, "Failed to create chat thread", http.StatusInternalServerError)
		return
	}

	response := dtos.ChatResponse{
		ThreadID: strconv.FormatUint(uint64(chatEntity.ID()), 10),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (c *AIChatController) getThreadMessages(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())
	chatID, err := strconv.ParseUint(mux.Vars(r)["chat_id"], 10, 32)
	if err != nil {
		logger.WithError(err).Error("invalid chat ID format")
		http.Error(w, "Invalid chat ID format", http.StatusBadRequest)
		return
	}

	chatEntity, err := c.chatService.GetByID(r.Context(), uint(chatID))
	if err != nil {
		logger.WithError(err).Error("failed to get chat by ID")
		http.Error(w, "Thread not found", http.StatusNotFound)
		return
	}

	messages := chatEntity.Messages()
	threadMessages := make([]dtos.ThreadMessage, 0, len(messages))
	for _, msg := range messages {
		var role string
		if msg.Sender().Sender().Type() == chat.ClientSenderType {
			role = "user"
		} else {
			role = "assistant"
		}
		threadMessages = append(threadMessages, dtos.ThreadMessage{
			Role:      role,
			Message:   msg.Message(),
			Timestamp: msg.CreatedAt().Format(time.RFC3339),
		})
	}

	response := dtos.ThreadMessagesResponse{
		Messages: threadMessages,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (c *AIChatController) addMessageToThread(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())

	var msg dtos.ChatMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		logger.WithError(err).Error("failed to decode request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	chatID, err := strconv.ParseUint(mux.Vars(r)["chat_id"], 10, 32)
	if err != nil {
		logger.WithError(err).Error("failed to parse chat ID")
		http.Error(w, "Invalid chat ID", http.StatusBadRequest)
		return
	}

	_, err = c.chatService.SendMessageToThread(r.Context(), websiteServices.SendMessageToThreadDTO{
		ChatID:  uint(chatID),
		Message: msg.Message,
	})
	if err != nil {
		logger.WithError(err).Error("failed to send message to chat thread")
		http.Error(w, "Failed to send message to chat thread", http.StatusInternalServerError)
		return
	}

	chatEntity, err := c.chatService.ReplyWithAI(r.Context(), uint(chatID))
	if err != nil {
		logger.WithError(err).Error("failed to get AI response")
		http.Error(w, "Failed to get AI response", http.StatusInternalServerError)
		return
	}

	response := dtos.ChatResponse{
		ThreadID: strconv.FormatUint(uint64(chatEntity.ID()), 10),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
