// Package eskiz provides this package.
package eskiz

import (
	"context"
	"net/http"
	"time"

	eskizapi "github.com/iota-uz/eskiz"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/eskiz/models"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/sirupsen/logrus"
)

const (
	apiTimeout = 30 * time.Second
)

// Service is the domain-level Eskiz client. It wraps the generated API and
// surfaces a stable, Go-idiomatic interface for SMS delivery, batch sending,
// status polling, balance inquiries, and template moderation.
//
// All methods return wrapped domain types (package models) rather than
// OpenAPI-generated structs so callers never couple to generated code.
type Service interface {
	// SendSMS sends a single SMS. Config's CallbackURL, if set, is
	// attached so the receiving webhook receives delivery events.
	SendSMS(ctx context.Context, model models.SendSMS) (models.SendSMSResult, error)

	// SendBatch submits up to ~200 rows in one call. Per-row delivery
	// events are fetched later via GetSMSStatus or via the webhook.
	SendBatch(ctx context.Context, messages []models.BatchMessage) (models.BatchResult, error)

	// GetSMSStatus returns the delivery status of a single SMS by its
	// Eskiz-assigned id (the value from SendSMSResult.ID()).
	GetSMSStatus(ctx context.Context, id string) (models.SMSStatus, error)

	// GetBalance returns the account's current credit balance.
	GetBalance(ctx context.Context) (models.Balance, error)

	// SubmitTemplate submits a template body for moderation.
	SubmitTemplate(ctx context.Context, body string) (models.TemplateSubmission, error)

	// ListTemplates returns all templates the account has ever submitted,
	// with current moderation status.
	ListTemplates(ctx context.Context) ([]models.TemplateRecord, error)
}

func NewService(
	cfg Config,
	logger *logrus.Logger,
	sdkConfig *configuration.Configuration,
) Service {
	httpClient := &http.Client{
		Timeout: apiTimeout,
	}

	// Create base client for token refresh (without auth)
	baseConfig := eskizapi.NewConfiguration()
	baseConfig.Servers = eskizapi.ServerConfigurations{{URL: cfg.URL()}}
	baseConfig.HTTPClient = httpClient
	baseClient := eskizapi.NewAPIClient(baseConfig)

	refresher := &tokenRefresher{
		cfg:    cfg,
		client: baseClient,
	}

	// Create log transport for request/response logging
	logTransport := middleware.NewLogTransport(
		logger,
		sdkConfig,
		true, // log request bodies
		true, // log response bodies
		"eskiz",
	)

	// Create authenticated client
	authClient := &http.Client{
		Timeout: apiTimeout,
		Transport: &authRoundTripper{
			Base:      logTransport,
			Refresher: refresher,
		},
	}

	config := eskizapi.NewConfiguration()
	config.Servers = eskizapi.ServerConfigurations{{URL: cfg.URL()}}
	config.HTTPClient = authClient

	client := eskizapi.NewAPIClient(config)

	return &service{
		cfg:    cfg,
		client: client,
	}
}

type service struct {
	cfg    Config
	client *eskizapi.APIClient
}

func (s *service) SendSMS(ctx context.Context, model models.SendSMS) (models.SendSMSResult, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}

	if model.PhoneNumber() == "" {
		return nil, ErrInvalidPhoneNumber
	}
	if model.Message() == "" {
		return nil, ErrInvalidMessage
	}
	if len(model.Message()) > s.cfg.MaxMessageSize() {
		return nil, ErrMessageTooLong
	}

	req := s.client.DefaultApi.
		SendSms(ctx).
		MobilePhone(model.PhoneNumber()).
		Message(model.Message())

	if model.From() != "" {
		req = req.From(model.From())
	}
	if s.cfg.CallbackURL() != "" {
		req = req.CallbackUrl(s.cfg.CallbackURL())
	}

	res, httpResp, err := req.Execute()
	drain(httpResp)
	if err != nil {
		return nil, err
	}

	return models.NewSendSMSResult(res), nil
}

func drain(httpResp *http.Response) {
	if httpResp != nil {
		_ = httpResp.Body.Close()
	}
}

// SendBatch submits a batch of messages. Phones are converted to numeric
// form; a leading "+" is stripped and anything non-digit errors out
// (models.ErrInvalidBatchPhone).
func (s *service) SendBatch(ctx context.Context, messages []models.BatchMessage) (models.BatchResult, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}
	if len(messages) == 0 {
		return nil, ErrInvalidMessage
	}

	inner := make([]eskizapi.SendSmsBatchRequestMessagesInner, 0, len(messages))
	for _, m := range messages {
		if m.PhoneNumber() == "" {
			return nil, ErrInvalidPhoneNumber
		}
		if m.Message() == "" {
			return nil, ErrInvalidMessage
		}
		row, err := models.ToEskizInner(m)
		if err != nil {
			return nil, err
		}
		inner = append(inner, row)
	}

	batchReq := eskizapi.SendSmsBatchRequest{Messages: inner}
	if s.cfg.CallbackURL() != "" {
		cb := s.cfg.CallbackURL()
		batchReq.CallbackUrl = &cb
	}

	res, httpResp, err := s.client.DefaultApi.
		SendSmsBatch(ctx).
		SendSmsBatchRequest(batchReq).
		Execute()
	drain(httpResp)
	if err != nil {
		return nil, err
	}
	return models.NewBatchResult(res), nil
}

// GetSMSStatus fetches the current delivery status for a single SMS by id.
func (s *service) GetSMSStatus(ctx context.Context, id string) (models.SMSStatus, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}
	if id == "" {
		return nil, ErrInvalidMessage
	}
	res, httpResp, err := s.client.DefaultApi.GetSmsStatusById(ctx, id).Execute()
	drain(httpResp)
	if err != nil {
		return nil, err
	}
	return models.NewSMSStatus(res), nil
}

// GetBalance returns the account's current credit balance.
func (s *service) GetBalance(ctx context.Context) (models.Balance, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}
	res, httpResp, err := s.client.DefaultApi.GetUserLimit(ctx).Execute()
	drain(httpResp)
	if err != nil {
		return nil, err
	}
	return models.NewBalance(res), nil
}

// SubmitTemplate queues a template body for moderation.
func (s *service) SubmitTemplate(ctx context.Context, body string) (models.TemplateSubmission, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}
	if body == "" {
		return nil, ErrInvalidMessage
	}
	if len(body) > s.cfg.MaxMessageSize() {
		return nil, ErrMessageTooLong
	}
	res, httpResp, err := s.client.DefaultApi.
		SendTemplate(ctx).
		Template(body).
		Execute()
	drain(httpResp)
	if err != nil {
		return nil, err
	}
	return models.NewTemplateSubmission(res), nil
}

// ListTemplates returns all submitted templates with their moderation status.
func (s *service) ListTemplates(ctx context.Context) ([]models.TemplateRecord, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}
	res, httpResp, err := s.client.DefaultApi.GetUserTemplates(ctx).Execute()
	drain(httpResp)
	if err != nil {
		return nil, err
	}
	return models.NewTemplateRecords(res), nil
}
