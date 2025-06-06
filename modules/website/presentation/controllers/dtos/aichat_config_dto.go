package dtos

import (
	"context"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/website/domain/entities/aichatconfig"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

type AIConfigDTO struct {
	ModelName    string  `validate:"required"`
	SystemPrompt string  `validate:"omitempty"`
	Temperature  float32 `validate:"omitempty,gte=0,lte=2"`
	MaxTokens    int     `validate:"omitempty,gt=0"`
	BaseURL      string  `validate:"required,url"`
	AccessToken  string  `validate:"omitempty"`
}

func (dto *AIConfigDTO) Apply(cfg aichatconfig.AIConfig, tenantID uuid.UUID) (aichatconfig.AIConfig, error) {
	if cfg == nil {
		options := []aichatconfig.Option{
			aichatconfig.WithTemperature(mapping.Or(dto.Temperature, 0.7)),
			aichatconfig.WithMaxTokens(mapping.Or(dto.MaxTokens, 1024)),
			aichatconfig.WithIsDefault(true),
			aichatconfig.WithTenantID(tenantID),
		}

		if dto.AccessToken != "" {
			options = append(options, aichatconfig.WithAccessToken(dto.AccessToken))
		}

		if dto.SystemPrompt != "" {
			options = append(options, aichatconfig.WithSystemPrompt(dto.SystemPrompt))
		}

		return aichatconfig.New(
			dto.ModelName,
			aichatconfig.AIModelTypeOpenAI,
			dto.BaseURL,
			options...,
		)
	}
	var err error
	if dto.ModelName != "" {
		cfg, err = cfg.WithModelName(dto.ModelName)
		if err != nil {
			return nil, err
		}
	}
	if dto.SystemPrompt != "" {
		cfg = cfg.SetSystemPrompt(dto.SystemPrompt)
	}
	if dto.Temperature != 0 {
		cfg, err = cfg.WithTemperature(dto.Temperature)
		if err != nil {
			return nil, err
		}
	}
	if dto.MaxTokens != 0 {
		cfg, err = cfg.WithMaxTokens(dto.MaxTokens)
		if err != nil {
			return nil, err
		}
	}
	if dto.BaseURL != "" {
		cfg, err = cfg.WithBaseURL(dto.BaseURL)
		if err != nil {
			return nil, err
		}
	}
	if dto.AccessToken != "" {
		cfg = cfg.SetAccessToken(dto.AccessToken)
	}
	return cfg, nil
}

func (dto *AIConfigDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
	}
	errorMessages := map[string]string{}
	errs := constants.Validate.Struct(dto)
	if errs == nil {
		return errorMessages, len(errorMessages) == 0
	}

	for _, err := range errs.(validator.ValidationErrors) {
		translatedFieldName := l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("AIChatBot.%s.Label", err.Field()),
		})
		errorMessages[err.Field()] = l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("ValidationErrors.%s", err.Tag()),
			TemplateData: map[string]string{
				"Field": translatedFieldName,
			},
		})
	}

	return errorMessages, len(errorMessages) == 0
}
