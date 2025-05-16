package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/website/domain/entities/aichatconfig"
	"github.com/iota-uz/iota-sdk/modules/website/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/website/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/website/presentation/templates/pages/aichat"
	"github.com/iota-uz/iota-sdk/modules/website/presentation/viewmodels"
	websiteServices "github.com/iota-uz/iota-sdk/modules/website/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/sirupsen/logrus"
)

type AIChatControllerConfig struct {
	BasePath string
	App      application.Application
}

type AIChatController struct {
	basePath string
	app      application.Application
}

func NewAIChatController(cfg AIChatControllerConfig) application.Controller {
	return &AIChatController{
		basePath: cfg.BasePath,
		app:      cfg.App,
	}
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
	router.HandleFunc("", di.H(c.configureAIChat)).Methods(http.MethodGet)
	router.HandleFunc("/config", di.H(c.saveConfig)).Methods(http.MethodPost)

	bareRouter := r.PathPrefix(c.basePath).Subrouter()
	bareRouter.Use(
		middleware.ProvideLocalizer(c.app.Bundle()),
	)
	bareRouter.HandleFunc("/messages", di.H(c.createThread)).Methods(http.MethodPost)
	bareRouter.HandleFunc("/messages/{thread_id}", di.H(c.getThreadMessages)).Methods(http.MethodGet)
	bareRouter.HandleFunc("/messages/{thread_id}", di.H(c.addMessageToThread)).Methods(http.MethodPost)
}

func (c *AIChatController) configureAIChat(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	configService *websiteServices.AIChatConfigService,
	localizer *i18n.Localizer,
) {
	// Try to get the default configuration
	config, err := configService.GetDefault(r.Context())
	if err != nil && !errors.Is(err, aichatconfig.ErrConfigNotFound) {
		logger.WithError(err).Error("failed to get default AI chat configuration")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		err = json.NewEncoder(w).Encode(dtos.NewAPIError(
			localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.FailedToGetConfig"}),
			dtos.ErrorCodeInternalServer,
		))
		if err != nil {
			panic(err)
		}
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
			Temperature: 0.7,
			MaxTokens:   1024,
			BaseURL:     "https://api.openai.com/v1",
		}
	}

	templ.Handler(aichat.Configure(props)).ServeHTTP(w, r)
}

func (c *AIChatController) saveConfig(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	configService *websiteServices.AIChatConfigService,
	localizer *i18n.Localizer,
) {
	dto, err := composables.UseForm(&dtos.AIConfigDTO{}, r)
	if err != nil {
		logger.WithError(err).Error("failed to parse form")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		err = json.NewEncoder(w).Encode(dtos.NewAPIError(
			localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.InvalidFormData"}),
			dtos.ErrorCodeInvalidRequest,
		))
		if err != nil {
			panic(err)
		}
		return
	}

	config, _ := configService.GetDefault(r.Context())
	if errors, ok := dto.Ok(r.Context()); !ok {
		logger.WithField("errors", errors).Error("validation failed")

		props := aichat.ConfigureProps{
			FormAction: c.basePath + "/config",
			Config: &viewmodels.AIConfig{
				ModelName:    dto.ModelName,
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		err = json.NewEncoder(w).Encode(dtos.NewAPIError(
			localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.InvalidFormData"}),
			dtos.ErrorCodeInvalidRequest,
		))
		if err != nil {
			panic(err)
		}
		return
	}

	_, err = configService.Save(r.Context(), configEntity)
	if err != nil {
		logger.WithError(err).Error("failed to save AI chat configuration")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		err = json.NewEncoder(w).Encode(dtos.NewAPIError(
			localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.FailedToSaveConfig"}),
			dtos.ErrorCodeInternalServer,
		))
		if err != nil {
			panic(err)
		}
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *AIChatController) createThread(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	chatService *websiteServices.WebsiteChatService,
	localizer *i18n.Localizer,
) {
	var msg dtos.ChatMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		logger.WithError(err).Error("failed to decode request body")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		err = json.NewEncoder(w).Encode(dtos.NewAPIError(
			localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.InvalidRequestBody"}),
			dtos.ErrorCodeInvalidRequest,
		))
		if err != nil {
			panic(err)
		}
		return
	}

	thread, err := chatService.CreateThread(r.Context(), websiteServices.CreateThreadDTO{
		Phone:   msg.Phone,
		Country: country.Uzbekistan,
	})
	if err != nil {
		logger.WithError(err).Error("failed to create chat thread")

		w.Header().Set("Content-Type", "application/json")
		if errors.Is(err, phone.ErrInvalidPhoneNumber) {
			w.WriteHeader(http.StatusBadRequest)
			err = json.NewEncoder(w).Encode(dtos.NewAPIError(
				localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.InvalidPhoneFormat"}),
				dtos.ErrorCodeInvalidPhoneFormat,
			))
			if err != nil {
				panic(err)
			}
			return
		} else if errors.Is(err, phone.ErrUnknownCountry) {
			w.WriteHeader(http.StatusBadRequest)
			err = json.NewEncoder(w).Encode(dtos.NewAPIError(
				localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.UnknownCountryCode"}),
				dtos.ErrorCodeUnknownCountryCode,
			))
			if err != nil {
				panic(err)
			}
			return
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			err = json.NewEncoder(w).Encode(dtos.NewAPIError(
				localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.FailedToCreateThread"}),
				dtos.ErrorCodeInternalServer,
			))
			if err != nil {
				panic(err)
			}
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&dtos.ChatResponse{
		ThreadID: thread.ID().String(),
	})
	if err != nil {
		panic(err)
	}
}

func (c *AIChatController) getThreadMessages(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	chatService *websiteServices.WebsiteChatService,
	localizer *i18n.Localizer,
) {
	threadID, err := uuid.Parse(mux.Vars(r)["thread_id"])
	if err != nil {
		logger.WithError(err).Error("invalid thread ID format")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		err = json.NewEncoder(w).Encode(dtos.NewAPIError(
			localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.InvalidThreadIDFormat"}),
			dtos.ErrorCodeInvalidRequest,
		))
		if err != nil {
			panic(err)
		}
		return
	}

	// Get the thread from the service
	thread, err := chatService.GetThreadByID(r.Context(), threadID)
	if err != nil {
		logger.WithError(err).Error("failed to get thread by ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		err = json.NewEncoder(w).Encode(dtos.NewAPIError(
			localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.ThreadNotFound"}),
			dtos.ErrorCodeThreadNotFound,
		))
		if err != nil {
			panic(err)
		}
		return
	}

	messages := thread.Messages()
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
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		panic(err)
	}
}

func (c *AIChatController) addMessageToThread(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	chatService *websiteServices.WebsiteChatService,
	localizer *i18n.Localizer,
) {
	var msg dtos.ChatMessage
	_ = json.NewDecoder(r.Body).Decode(&msg)

	threadID, err := uuid.Parse(mux.Vars(r)["thread_id"])
	if err != nil {
		logger.WithError(err).Error("invalid thread ID format")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		err = json.NewEncoder(w).Encode(dtos.NewAPIError(
			localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.InvalidThreadIDFormat"}),
			dtos.ErrorCodeInvalidRequest,
		))
		if err != nil {
			panic(err)
		}
		return
	}

	_, err = chatService.GetThreadByID(r.Context(), threadID)
	if err != nil {
		logger.WithError(err).Error("failed to get thread by ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		err = json.NewEncoder(w).Encode(dtos.NewAPIError(
			localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.ThreadNotFound"}),
			dtos.ErrorCodeThreadNotFound,
		))
		if err != nil {
			panic(err)
		}
		return
	}

	_, err = chatService.SendMessageToThread(r.Context(), websiteServices.SendMessageToThreadDTO{
		ThreadID: threadID,
		Message:  msg.Message,
	})
	if err != nil {
		logger.WithError(err).Error("failed to send message to chat thread")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		err = json.NewEncoder(w).Encode(dtos.NewAPIError(
			localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.FailedToSendMessage"}),
			dtos.ErrorCodeInternalServer,
		))
		if err != nil {
			panic(err)
		}
		return
	}

	aiResponseThread, err := chatService.ReplyWithAI(r.Context(), threadID)
	if err != nil {
		logger.WithError(err).Error("failed to get AI response")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		err = json.NewEncoder(w).Encode(dtos.NewAPIError(
			localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.FailedToGetAIResponse"}),
			dtos.ErrorCodeInternalServer,
		))
		if err != nil {
			panic(err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&dtos.ChatResponse{
		ThreadID: aiResponseThread.ID().String(),
	})
	if err != nil {
		panic(err)
	}
}
