package controllers

import (
	"errors"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/go-i18n/v2/i18n"
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
}

func (c *AIChatController) configureAIChat(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	configService *websiteServices.AIChatConfigService,
	chatService *websiteServices.WebsiteChatService,
	localizer *i18n.Localizer,
) {
	config, err := configService.GetDefault(r.Context())
	if err != nil && !errors.Is(err, aichatconfig.ErrConfigNotFound) {
		logger.WithError(err).Error("failed to get default AI chat configuration")
		writeJSONError(w, http.StatusInternalServerError,
			localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.FailedToGetConfig"}),
			dtos.ErrorCodeInternalServer)
		return
	}

	modelOptions, err := c.fetchModelOptions(r, logger, chatService, config, localizer, w)
	if err != nil {
		return
	}

	props := buildConfigureProps(c.basePath, config, modelOptions)
	templ.Handler(aichat.Configure(props)).ServeHTTP(w, r)
}

func (c *AIChatController) fetchModelOptions(
	r *http.Request,
	logger *logrus.Entry,
	chatService *websiteServices.WebsiteChatService,
	config aichatconfig.AIConfig,
	localizer *i18n.Localizer,
	w http.ResponseWriter,
) ([]string, error) {
	if config == nil {
		return nil, nil
	}

	models, err := chatService.GetAvailableModels(r.Context())
	if err != nil {
		logger.WithError(err).Error("failed to get available models")
		writeJSONError(w, http.StatusInternalServerError,
			localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.FailedToGetModels"}),
			dtos.ErrorCodeInternalServer)
		return nil, err
	}
	return models, nil
}

func buildConfigureProps(basePath string, config aichatconfig.AIConfig, modelOptions []string) aichat.ConfigureProps {
	props := aichat.ConfigureProps{
		FormAction:   basePath + "/config",
		ModelOptions: modelOptions,
	}

	if config != nil {
		props.Config = mappers.AIConfigToViewModel(config)
	} else {
		props.Config = &viewmodels.AIConfig{
			Temperature: 0.7,
			MaxTokens:   1024,
			BaseURL:     "https://api.openai.com/v1",
		}
	}

	return props
}

func (c *AIChatController) saveConfig(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	configService *websiteServices.AIChatConfigService,
	chatService *websiteServices.WebsiteChatService,
	localizer *i18n.Localizer,
) {
	dto, err := composables.UseForm(&dtos.AIConfigDTO{}, r)
	if err != nil {
		logger.WithError(err).Error("failed to parse form")
		writeJSONError(w, http.StatusBadRequest,
			localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.InvalidFormData"}),
			dtos.ErrorCodeInvalidRequest)
		return
	}

	config, _ := configService.GetDefault(r.Context())

	if errors, ok := dto.Ok(r.Context()); !ok {
		c.handleValidationErrors(w, r, logger, config, chatService, localizer, dto, errors)
		return
	}

	if err := c.persistConfig(r, logger, configService, localizer, dto, config, w); err != nil {
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *AIChatController) handleValidationErrors(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	config aichatconfig.AIConfig,
	chatService *websiteServices.WebsiteChatService,
	localizer *i18n.Localizer,
	dto *dtos.AIConfigDTO,
	errors map[string]string,
) {
	logger.WithField("errors", errors).Error("validation failed")

	modelOptions, err := c.fetchModelOptions(r, logger, chatService, config, localizer, w)
	if err != nil {
		return
	}

	props := buildValidationErrorProps(c.basePath, dto, config, modelOptions, errors)
	templ.Handler(aichat.ConfigureForm(props)).ServeHTTP(w, r)
}

func buildValidationErrorProps(
	basePath string,
	dto *dtos.AIConfigDTO,
	config aichatconfig.AIConfig,
	modelOptions []string,
	errors map[string]string,
) aichat.ConfigureProps {
	props := aichat.ConfigureProps{
		FormAction:   basePath + "/config",
		ModelOptions: modelOptions,
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

	return props
}

func (c *AIChatController) persistConfig(
	r *http.Request,
	logger *logrus.Entry,
	configService *websiteServices.AIChatConfigService,
	localizer *i18n.Localizer,
	dto *dtos.AIConfigDTO,
	config aichatconfig.AIConfig,
	w http.ResponseWriter,
) error {
	tenant, err := composables.UseTenant(r.Context())
	if err != nil {
		panic(err)
	}

	configEntity, err := dto.Apply(config, tenant.ID)
	if err != nil {
		logger.WithError(err).Error("failed to convert DTO to entity")
		writeJSONError(w, http.StatusBadRequest,
			localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.InvalidFormData"}),
			dtos.ErrorCodeInvalidRequest)
		return err
	}

	_, err = configService.Save(r.Context(), configEntity)
	if err != nil {
		logger.WithError(err).Error("failed to save AI chat configuration")
		writeJSONError(w, http.StatusInternalServerError,
			localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "AIChatBot.Errors.FailedToSaveConfig"}),
			dtos.ErrorCodeInternalServer)
		return err
	}

	return nil
}
