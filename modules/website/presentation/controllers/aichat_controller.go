package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	crmServices "github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/modules/website/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/website/presentation/templates/pages/aichat"
	websiteServices "github.com/iota-uz/iota-sdk/modules/website/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

func NewAIChatController(app application.Application) application.Controller {
	return &AIChatController{
		basePath:      "/website/ai-chat",
		app:           app,
		chatService:   app.Service(websiteServices.WebsiteChatService{}).(*websiteServices.WebsiteChatService),
		clientService: app.Service(crmServices.ClientService{}).(*crmServices.ClientService),
	}
}

type AIChatController struct {
	basePath      string
	app           application.Application
	chatService   *websiteServices.WebsiteChatService
	clientService *crmServices.ClientService
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

	bareRouter := r.PathPrefix(c.basePath).Subrouter()
	bareRouter.HandleFunc("/payload", c.aiChat).Methods(http.MethodGet)
	bareRouter.HandleFunc("/test-wc", c.aiChatWC).Methods(http.MethodGet)
	bareRouter.HandleFunc("/message", c.createThread).Methods(http.MethodPost)
	bareRouter.HandleFunc("/messages/{chat_id}", c.getThreadMessages).Methods(http.MethodGet)
	bareRouter.HandleFunc("/messages/{chat_id}", c.addMessageToThread).Methods(http.MethodPost)
}

func (c *AIChatController) configureAIChat(w http.ResponseWriter, r *http.Request) {
	templ.Handler(aichat.Configure(aichat.Props{
		Title:       "AI Chatbot",
		Description: "Наш AI-бот готов помочь вам круглосуточно",
		Endpoint:    c.basePath + "/message",
	})).ServeHTTP(w, r)
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

	chatEntity, err := c.chatService.CreateThread(r.Context(), msg.Contact)
	if err != nil {
		logger.WithError(err).Error("failed to create chat thread")
		http.Error(w, "Failed to create chat thread", http.StatusInternalServerError)
		return
	}

	chatEntity, err = c.chatService.SendMessageToThread(r.Context(), websiteServices.SendMessageToThreadDTO{
		ChatID:  chatEntity.ID(),
		Message: msg.Message,
	})

	if err != nil {
		logger.WithError(err).Error("failed to send message to chat thread")
		http.Error(w, "Failed to send message to chat thread", http.StatusInternalServerError)
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
			Role:    role,
			Message: msg.Message(),
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

	chatEntity, err := c.chatService.SendMessageToThread(r.Context(), websiteServices.SendMessageToThreadDTO{
		ChatID:  uint(chatID),
		Message: msg.Message,
	})

	response := dtos.ChatResponse{
		ThreadID: strconv.FormatUint(uint64(chatEntity.ID()), 10),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
