package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/website/presentation/controllers/dtos"
	websiteServices "github.com/iota-uz/iota-sdk/modules/website/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/sirupsen/logrus"
)

type AIChatAPIControllerConfig struct {
	BasePath string
	App      application.Application
}

type AIChatAPIController struct {
	basePath string
	app      application.Application
}

func NewAIChatAPIController(cfg AIChatAPIControllerConfig) application.Controller {
	return &AIChatAPIController{
		basePath: cfg.BasePath,
		app:      cfg.App,
	}
}

func (c *AIChatAPIController) Key() string {
	return "AIChatAPIController"
}

func (c *AIChatAPIController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(middleware.ProvideLocalizer(c.app.Bundle()))

	router.HandleFunc("/messages", di.H(c.createThread)).Methods(http.MethodPost)
	router.HandleFunc("/messages/{thread_id}", di.H(c.getThreadMessages)).Methods(http.MethodGet)
	router.HandleFunc("/messages/{thread_id}", di.H(c.addMessageToThread)).Methods(http.MethodPost)
}

func (c *AIChatAPIController) createThread(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	chatService *websiteServices.WebsiteChatService,
	localizer *i18n.Localizer,
) {
	var msg dtos.ChatMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		logger.WithError(err).Error("failed to decode request body")
		writeJSONError(w, http.StatusBadRequest,
			localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.InvalidRequestBody"}),
			dtos.ErrorCodeInvalidRequest)
		return
	}

	thread, err := chatService.CreateThread(r.Context(), websiteServices.CreateThreadDTO{
		Phone:   msg.Phone,
		Country: country.Uzbekistan,
	})

	if err != nil {
		c.handleThreadCreationError(w, err, logger, localizer)
		return
	}

	writeJSON(w, &dtos.ChatResponse{
		ThreadID: thread.ID().String(),
	})
}

func (c *AIChatAPIController) handleThreadCreationError(
	w http.ResponseWriter,
	err error,
	logger *logrus.Entry,
	localizer *i18n.Localizer,
) {
	logger.WithError(err).Error("failed to create chat thread")

	switch {
	case errors.Is(err, phone.ErrInvalidPhoneNumber):
		writeJSONError(w, http.StatusBadRequest,
			localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.InvalidPhoneFormat"}),
			dtos.ErrorCodeInvalidPhoneFormat)
	case errors.Is(err, phone.ErrUnknownCountry):
		writeJSONError(w, http.StatusBadRequest,
			localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.UnknownCountryCode"}),
			dtos.ErrorCodeUnknownCountryCode)
	default:
		writeJSONError(w, http.StatusInternalServerError,
			localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.FailedToCreateThread"}),
			dtos.ErrorCodeInternalServer)
	}
}

func (c *AIChatAPIController) getThreadMessages(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	chatService *websiteServices.WebsiteChatService,
	localizer *i18n.Localizer,
) {
	threadID, err := parseThreadID(r)
	if err != nil {
		logger.WithError(err).Error("invalid thread ID format")
		writeJSONError(w, http.StatusBadRequest,
			localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.InvalidThreadIDFormat"}),
			dtos.ErrorCodeInvalidRequest)
		return
	}

	thread, err := chatService.GetThreadByID(r.Context(), threadID)
	if err != nil {
		logger.WithError(err).Error("failed to get thread by ID")
		writeJSONError(w, http.StatusNotFound,
			localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.ThreadNotFound"}),
			dtos.ErrorCodeThreadNotFound)
		return
	}

	messages := transformThreadMessages(thread.Messages())
	writeJSON(w, dtos.ThreadMessagesResponse{Messages: messages})
}

func transformThreadMessages(messages []chat.Message) []dtos.ThreadMessage {
	threadMessages := make([]dtos.ThreadMessage, 0, len(messages))
	for _, msg := range messages {
		role := "assistant"
		if msg.Sender().Sender().Type() == chat.ClientSenderType {
			role = "user"
		}

		threadMessages = append(threadMessages, dtos.ThreadMessage{
			Role:      role,
			Message:   msg.Message(),
			Timestamp: msg.CreatedAt().Format(time.RFC3339),
		})
	}
	return threadMessages
}

func (c *AIChatAPIController) addMessageToThread(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	chatService *websiteServices.WebsiteChatService,
	localizer *i18n.Localizer,
) {
	threadID, err := parseThreadID(r)
	if err != nil {
		logger.WithError(err).Error("invalid thread ID format")
		writeJSONError(w, http.StatusBadRequest,
			localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.InvalidThreadIDFormat"}),
			dtos.ErrorCodeInvalidRequest)
		return
	}

	var msg dtos.ChatMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		logger.WithError(err).Error("failed to decode request body")
		writeJSONError(w, http.StatusBadRequest,
			localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.InvalidRequestBody"}),
			dtos.ErrorCodeInvalidRequest)
		return
	}

	if err := c.validateThreadExists(r.Context(), threadID, chatService); err != nil {
		logger.WithError(err).Error("thread not found")
		writeJSONError(w, http.StatusNotFound,
			localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.ThreadNotFound"}),
			dtos.ErrorCodeThreadNotFound)
		return
	}

	if err := c.sendAndReplyToMessage(r.Context(), threadID, msg.Message, chatService); err != nil {
		logger.WithError(err).Error("failed to process message")
		writeJSONError(w, http.StatusInternalServerError,
			localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.FailedToSendMessage"}),
			dtos.ErrorCodeInternalServer)
		return
	}

	writeJSON(w, &dtos.ChatResponse{
		ThreadID: threadID.String(),
	})
}

func (c *AIChatAPIController) validateThreadExists(ctx context.Context, threadID uuid.UUID, chatService *websiteServices.WebsiteChatService) error {
	_, err := chatService.GetThreadByID(ctx, threadID)
	return err
}

func (c *AIChatAPIController) sendAndReplyToMessage(ctx context.Context, threadID uuid.UUID, message string, chatService *websiteServices.WebsiteChatService) error {
	_, err := chatService.SendMessageToThread(ctx, websiteServices.SendMessageToThreadDTO{
		ThreadID: threadID,
		Message:  message,
	})
	if err != nil {
		return err
	}

	_, err = chatService.ReplyWithAI(ctx, threadID)
	return err
}

func parseThreadID(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(mux.Vars(r)["thread_id"])
}
