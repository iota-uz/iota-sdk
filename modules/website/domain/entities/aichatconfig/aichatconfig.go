package aichatconfig

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidID          = errors.New("invalid ID")
	ErrInvalidTemperature = errors.New("temperature must be between 0.0 and 2.0")
	ErrEmptyModelName     = errors.New("empty model name")
	ErrEmptyBaseURL       = errors.New("empty base URL")
	ErrConfigNotFound     = errors.New("AI chat configuration not found")
)

type AIModelType string

const (
	AIModelTypeOpenAI AIModelType = "openai"
)

type AIConfig interface {
	ID() uuid.UUID
	TenantID() uuid.UUID
	ModelName() string
	ModelType() AIModelType
	SystemPrompt() string
	Temperature() float32
	MaxTokens() int
	BaseURL() string
	AccessToken() string
	IsDefault() bool
	CreatedAt() time.Time
	UpdatedAt() time.Time
	SetSystemPrompt(prompt string) AIConfig
	WithTemperature(temp float32) (AIConfig, error)
	WithMaxTokens(tokens int) (AIConfig, error)
	WithModelName(modelName string) (AIConfig, error)
	WithBaseURL(baseURL string) (AIConfig, error)
	SetAccessToken(accessToken string) AIConfig
	WithIsDefault(isDefault bool) (AIConfig, error)
}

type Repository interface {
	GetByID(ctx context.Context, id uuid.UUID) (AIConfig, error)
	GetDefault(ctx context.Context) (AIConfig, error)
	Save(ctx context.Context, config AIConfig) (AIConfig, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context) ([]AIConfig, error)
	SetDefault(ctx context.Context, id uuid.UUID) error
}

type aiConfig struct {
	id           uuid.UUID
	tenantID     uuid.UUID
	modelName    string
	modelType    AIModelType
	systemPrompt string
	temperature  float32
	maxTokens    int
	baseURL      string
	accessToken  string
	isDefault    bool
	createdAt    time.Time
	updatedAt    time.Time
}

func MustNew(
	modelName string,
	modelType AIModelType,
	baseURL string,
	opts ...Option,
) AIConfig {
	cfg, err := New(modelName, modelType, baseURL, opts...)
	if err != nil {
		panic(err)
	}
	return cfg
}

func New(
	modelName string,
	modelType AIModelType,
	baseURL string,
	opts ...Option,
) (AIConfig, error) {
	if modelName == "" {
		return nil, ErrEmptyModelName
	}

	if baseURL == "" {
		return nil, ErrEmptyBaseURL
	}

	cfg := &aiConfig{
		id:           uuid.New(),
		tenantID:     uuid.Nil, // Will be set via WithTenantID option
		modelName:    modelName,
		modelType:    modelType,
		systemPrompt: "",
		baseURL:      baseURL,
		accessToken:  "",   // No default access token
		temperature:  0.7,  // Default temperature
		maxTokens:    1024, // Default max tokens
		createdAt:    time.Now(),
		updatedAt:    time.Now(),
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return cfg, nil
}

type Option func(*aiConfig)

func WithID(id uuid.UUID) Option {
	return func(c *aiConfig) {
		if id != uuid.Nil {
			c.id = id
		}
	}
}

func WithTenantID(tenantID uuid.UUID) Option {
	return func(c *aiConfig) {
		if tenantID != uuid.Nil {
			c.tenantID = tenantID
		}
	}
}

func WithSystemPrompt(prompt string) Option {
	return func(c *aiConfig) {
		c.systemPrompt = prompt
	}
}

func WithTemperature(temp float32) Option {
	return func(c *aiConfig) {
		if temp >= 0.0 && temp <= 2.0 {
			c.temperature = temp
		}
	}
}

func WithMaxTokens(tokens int) Option {
	return func(c *aiConfig) {
		if tokens > 0 {
			c.maxTokens = tokens
		}
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(c *aiConfig) {
		if !createdAt.IsZero() {
			c.createdAt = createdAt
		}
	}
}

func WithUpdatedAt(updatedAt time.Time) Option {
	return func(c *aiConfig) {
		if !updatedAt.IsZero() {
			c.updatedAt = updatedAt
		}
	}
}

func WithBaseURL(baseURL string) Option {
	return func(c *aiConfig) {
		if baseURL != "" {
			c.baseURL = baseURL
		}
	}
}

func WithAccessToken(accessToken string) Option {
	return func(c *aiConfig) {
		c.accessToken = accessToken
	}
}

func WithIsDefault(isDefault bool) Option {
	return func(c *aiConfig) {
		c.isDefault = isDefault
	}
}

func (c *aiConfig) ID() uuid.UUID {
	return c.id
}

func (c *aiConfig) TenantID() uuid.UUID {
	return c.tenantID
}

func (c *aiConfig) ModelName() string {
	return c.modelName
}

func (c *aiConfig) ModelType() AIModelType {
	return c.modelType
}

func (c *aiConfig) SystemPrompt() string {
	return c.systemPrompt
}

func (c *aiConfig) Temperature() float32 {
	return c.temperature
}

func (c *aiConfig) MaxTokens() int {
	return c.maxTokens
}

func (c *aiConfig) BaseURL() string {
	return c.baseURL
}

func (c *aiConfig) AccessToken() string {
	return c.accessToken
}

func (c *aiConfig) CreatedAt() time.Time {
	return c.createdAt
}

func (c *aiConfig) UpdatedAt() time.Time {
	return c.updatedAt
}

func (c *aiConfig) IsDefault() bool {
	return c.isDefault
}

func (c *aiConfig) SetSystemPrompt(prompt string) AIConfig {
	newConfig := *c
	newConfig.systemPrompt = prompt
	newConfig.updatedAt = time.Now()

	return &newConfig
}

func (c *aiConfig) WithTemperature(temp float32) (AIConfig, error) {
	if temp < 0.0 || temp > 2.0 {
		return nil, ErrInvalidTemperature
	}

	newConfig := *c
	newConfig.temperature = temp
	newConfig.updatedAt = time.Now()

	return &newConfig, nil
}

func (c *aiConfig) WithMaxTokens(tokens int) (AIConfig, error) {
	if tokens <= 0 {
		return nil, errors.New("max tokens must be positive")
	}

	newConfig := *c
	newConfig.maxTokens = tokens
	newConfig.updatedAt = time.Now()

	return &newConfig, nil
}

func (c *aiConfig) WithModelName(modelName string) (AIConfig, error) {
	if modelName == "" {
		return nil, ErrEmptyModelName
	}

	newConfig := *c
	newConfig.modelName = modelName
	newConfig.updatedAt = time.Now()

	return &newConfig, nil
}

func (c *aiConfig) WithBaseURL(baseURL string) (AIConfig, error) {
	if baseURL == "" {
		return nil, ErrEmptyBaseURL
	}

	newConfig := *c
	newConfig.baseURL = baseURL
	newConfig.updatedAt = time.Now()

	return &newConfig, nil
}

func (c *aiConfig) SetAccessToken(accessToken string) AIConfig {
	newConfig := *c
	newConfig.accessToken = accessToken
	newConfig.updatedAt = time.Now()

	return &newConfig
}

func (c *aiConfig) WithIsDefault(isDefault bool) (AIConfig, error) {
	newConfig := *c
	newConfig.isDefault = isDefault
	newConfig.updatedAt = time.Now()

	return &newConfig, nil
}
