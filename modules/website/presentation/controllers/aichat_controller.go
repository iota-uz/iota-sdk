package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/modules/website/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/website/presentation/templates/pages/aichat"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

func NewAIChatController(app application.Application) application.Controller {
	return &AIChatController{
		basePath:      "/website/ai-chat",
		app:           app,
		chatService:   app.Service(services.ChatService{}).(*services.ChatService),
		clientService: app.Service(services.ClientService{}).(*services.ClientService),
	}
}

type AIChatController struct {
	basePath      string
	app           application.Application
	chatService   *services.ChatService
	clientService *services.ClientService
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
	bareRouter.HandleFunc("/message", c.handleMessage).Methods(http.MethodPost)
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

func (c *AIChatController) handleMessage(w http.ResponseWriter, r *http.Request) {
	// Parse the incoming message
	var msg dtos.ChatMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	logger := composables.UseLogger(ctx)

	var chatEntity chat.Chat
	var member chat.Member
	var err error

	// Get or create a chat based on phone number if provided
	if msg.Phone != "" {
		logger.WithField("phone", msg.Phone).Info("Using phone number to get or create chat")
		chatEntity, _, err = c.chatService.GetOrCreateChatByPhone(ctx, msg.Phone)
		if err != nil {
			logger.WithError(err).Error("failed to get or create chat by phone")
			http.Error(w, "Failed to get or create chat", http.StatusInternalServerError)
			return
		}
		member, err = c.chatService.GetMemberByContact(ctx, client.ContactTypePhone, msg.Phone)
		if err != nil {
			if errors.Is(err, persistence.ErrMemberNotFound) {
				member = chat.NewMember(
					chat.NewClientSender(chat.WebsiteTransport, chatEntity.ClientID(), "", ""),
				)
			} else {
				logger.WithError(err).Error("failed to get member by phone")
				http.Error(w, "Failed to get member by phone", http.StatusInternalServerError)
				return
			}
		}
	} else {
		http.Error(w, "Phone number is required", http.StatusBadRequest)
		return
	}

	// Add the client message to the chat using service
	updatedChat, err := c.chatService.AddMessageToChat(
		ctx,
		chatEntity.ID(),
		chat.NewMessage(msg.Message, member),
	)
	if err != nil {
		logger.WithError(err).Error("failed to add message to chat")
		http.Error(w, "Failed to add message to chat", http.StatusInternalServerError)
		return
	}

	// Add an AI response message
	aiResponse := "Thank you for your message. I'll get back to you shortly."
	finalChat, err := c.chatService.AddMessageToChat(
		ctx,
		updatedChat.ID(),
		chat.NewMessage(aiResponse, member),
	)
	if err != nil {
		logger.WithError(err).Error("failed to add AI response")
		http.Error(w, "Failed to add AI response", http.StatusInternalServerError)
		return
	}

	// Create a response with a thread ID (using chat ID)
	response := dtos.ChatResponse{
		ThreadID: strconv.FormatUint(uint64(finalChat.ID()), 10),
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")

	// Write the response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (c *AIChatController) getThreadMessages(w http.ResponseWriter, r *http.Request) {
	// Get the chat ID from the URL path
	chatIDStr := mux.Vars(r)["chat_id"]
	logger := composables.UseLogger(r.Context())

	// Convert chat ID to uint
	chatID, err := strconv.ParseUint(chatIDStr, 10, 32)
	if err != nil {
		logger.WithError(err).WithField("chat_id", chatIDStr).Error("invalid chat ID format")
		http.Error(w, "Invalid chat ID format", http.StatusBadRequest)
		return
	}

	// Get the chat by ID using the service
	chatEntity, err := c.chatService.GetByID(r.Context(), uint(chatID))
	if err != nil {
		http.Error(w, "Thread not found", http.StatusNotFound)
		return
	}

	// Convert chat messages to thread messages
	messages := chatEntity.Messages()
	threadMessages := make([]dtos.ThreadMessage, 0, len(messages))
	for _, msg := range messages {
		// TODO: Check if the sender is a client or AI
		role := "assistant"
		threadMessages = append(threadMessages, dtos.ThreadMessage{
			Role:    role,
			Message: msg.Message(),
		})
	}

	// Create the response
	response := dtos.ThreadMessagesResponse{
		Messages: threadMessages,
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")

	// Write the response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (c *AIChatController) addMessageToThread(w http.ResponseWriter, r *http.Request) {
	// Get the chat ID from the URL path
	vars := mux.Vars(r)
	chatIDStr := vars["chat_id"]
	logger := composables.UseLogger(r.Context())

	// Parse the incoming message
	var msg dtos.ChatMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Convert chat ID to uint
	chatID, err := strconv.ParseUint(chatIDStr, 10, 32)
	if err != nil {
		logger.WithError(err).WithField("chat_id", chatIDStr).Error("invalid chat ID format")
		http.Error(w, "Invalid chat ID format", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Create a sender based on message source
	var clientID uint

	// If phone provided, use it to identify the client
	if msg.Phone != "" {
		// Get client by phone first
		clientEntity, err := c.clientService.GetByPhone(ctx, msg.Phone)
		if err != nil {
			chatEntity, clientEntity, err := c.chatService.GetOrCreateChatByPhone(ctx, msg.Phone)
			if err != nil {
				logger.WithError(err).Error("failed to get or create chat by phone")
				http.Error(w, "Failed to get or create chat by phone", http.StatusInternalServerError)
				return
			}

			// Verify this client matches the requested chat
			if chatEntity.ID() != uint(chatID) {
				logger.Error("chat ID mismatch")
				http.Error(w, "Invalid chat ID for this phone number", http.StatusBadRequest)
				return
			}

			clientID = clientEntity.ID()
		} else {
			clientID = clientEntity.ID()
		}
	}

	member := chat.NewMember(
		chat.NewClientSender(chat.WebsiteTransport, clientID, "", ""),
	)
	// Add the client message to the chat
	updatedChat, err := c.chatService.AddMessageToChat(
		ctx,
		uint(chatID),
		chat.NewMessage(msg.Message, member),
	)
	if err != nil {
		logger.WithError(err).Error("failed to add message to chat")
		http.Error(w, "Failed to add message to chat", http.StatusInternalServerError)
		return
	}

	// Add an AI response message
	aiResponse := "Thank you for your message. I'll get back to you shortly."
	finalChat, err := c.chatService.AddMessageToChat(
		ctx,
		updatedChat.ID(),
		chat.NewMessage(
			aiResponse,
			chat.NewMember(
				chat.NewUserSender(chat.WebsiteTransport, 1, "AI", "AI"),
			),
		),
	)
	if err != nil {
		logger.WithError(err).Error("failed to add AI response")
		http.Error(w, "Failed to add AI response", http.StatusInternalServerError)
		return
	}

	// Convert chat messages to thread messages for response
	messages := finalChat.Messages()
	threadMessages := make([]dtos.ThreadMessage, 0, len(messages))
	for _, msg := range messages {
		// TODO: Check if the sender is a client or AI
		role := "assistant"
		threadMessages = append(threadMessages, dtos.ThreadMessage{
			Role:    role,
			Message: msg.Message(),
		})
	}

	// Create the response with the updated thread
	response := dtos.ThreadMessagesResponse{
		Messages: threadMessages,
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")

	// Write the response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
