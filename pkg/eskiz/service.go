// Package eskiz wraps Eskiz's SMS / template moderation REST API behind a
// stable Go interface; consumers never touch the generated OpenAPI client.
package eskiz

import (
	"context"
	"io"
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

type Service interface {
	SendSMS(ctx context.Context, model models.SendSMS) (models.SendSMSResult, error)
	// SendBatch posts up to ~200 rows in one call. Per-row delivery events
	// arrive via webhook or GetSMSStatus. Use SendBatchWithFrom to set the
	// sender id for the dispatch.
	SendBatch(ctx context.Context, messages []models.BatchMessage, opts ...models.SendBatchOption) (models.BatchResult, error)
	GetSMSStatus(ctx context.Context, id string) (models.SMSStatus, error)
	GetBalance(ctx context.Context) (models.Balance, error)
	SubmitTemplate(ctx context.Context, body string) (models.TemplateSubmission, error)
	ListTemplates(ctx context.Context) ([]models.TemplateRecord, error)
}

func NewService(
	cfg Config,
	logger *logrus.Logger,
	sdkConfig *configuration.Configuration,
) Service {
	httpClient := &http.Client{Timeout: apiTimeout}

	// Unauthenticated client used by the token refresher to obtain a token
	// without recursing back through itself.
	baseConfig := eskizapi.NewConfiguration()
	baseConfig.Servers = eskizapi.ServerConfigurations{{URL: cfg.URL()}}
	baseConfig.HTTPClient = httpClient
	refresher := &tokenRefresher{cfg: cfg, client: eskizapi.NewAPIClient(baseConfig)}

	logTransport := middleware.NewLogTransport(logger, sdkConfig, true, true, "eskiz")
	authClient := &http.Client{
		Timeout:   apiTimeout,
		Transport: &authRoundTripper{Base: logTransport, Refresher: refresher},
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
	if res == nil {
		return nil, ErrNilResponse
	}
	return models.NewSendSMSResult(res), nil
}

// drain consumes the body to EOF before close so http.Transport returns the
// connection to the keep-alive pool.
func drain(httpResp *http.Response) {
	if httpResp == nil || httpResp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, httpResp.Body)
	_ = httpResp.Body.Close()
}

func (s *service) SendBatch(ctx context.Context, messages []models.BatchMessage, opts ...models.SendBatchOption) (models.BatchResult, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}
	if len(messages) == 0 {
		return nil, ErrInvalidMessage
	}

	inner := make([]eskizapi.SendSmsBatchRequestMessagesInner, 0, len(messages))
	for _, m := range messages {
		if m.UserSmsID() == "" {
			return nil, ErrInvalidMessage
		}
		if m.PhoneNumber() == "" {
			return nil, ErrInvalidPhoneNumber
		}
		if m.Message() == "" {
			return nil, ErrInvalidMessage
		}
		if len(m.Message()) > s.cfg.MaxMessageSize() {
			return nil, ErrMessageTooLong
		}
		row, err := models.ToEskizInner(m)
		if err != nil {
			return nil, err
		}
		inner = append(inner, row)
	}

	o := models.SendBatchOptions{}
	for _, opt := range opts {
		opt(&o)
	}

	batchReq := eskizapi.SendSmsBatchRequest{Messages: inner}
	if s.cfg.CallbackURL() != "" {
		cb := s.cfg.CallbackURL()
		batchReq.CallbackUrl = &cb
	}
	if o.From != "" {
		from := o.From
		batchReq.From = &from
	}

	res, httpResp, err := s.client.DefaultApi.
		SendSmsBatch(ctx).
		SendSmsBatchRequest(batchReq).
		Execute()
	drain(httpResp)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, ErrNilResponse
	}
	return models.NewBatchResult(res), nil
}

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
	if res == nil {
		return nil, ErrNilResponse
	}
	return models.NewSMSStatus(res), nil
}

func (s *service) GetBalance(ctx context.Context) (models.Balance, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}
	res, httpResp, err := s.client.DefaultApi.GetUserLimit(ctx).Execute()
	drain(httpResp)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, ErrNilResponse
	}
	return models.NewBalance(res), nil
}

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
	if res == nil {
		return nil, ErrNilResponse
	}
	return models.NewTemplateSubmission(res), nil
}

func (s *service) ListTemplates(ctx context.Context) ([]models.TemplateRecord, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}
	res, httpResp, err := s.client.DefaultApi.GetUserTemplates(ctx).Execute()
	drain(httpResp)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, ErrNilResponse
	}
	return models.NewTemplateRecords(res), nil
}
