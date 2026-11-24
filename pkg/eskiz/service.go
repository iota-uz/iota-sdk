// Package eskiz wraps Eskiz's SMS / template moderation REST API behind a
// stable Go interface; consumers never touch the generated OpenAPI client.
package eskiz

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"time"

	eskizapi "github.com/iota-uz/eskiz"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/headers"
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
	GetMessagesByDispatch(ctx context.Context, dispatchID int64) ([]models.MessageStatus, error)
	GetSMSStatus(ctx context.Context, id string) (models.SMSStatus, error)
	GetBalance(ctx context.Context) (models.Balance, error)
	SubmitTemplate(ctx context.Context, body string) (models.TemplateSubmission, error)
	ListTemplates(ctx context.Context) ([]models.TemplateRecord, error)
}

// ServiceOption customizes NewService.
type ServiceOption func(*serviceOptions)

type serviceOptions struct {
	base        http.RoundTripper
	noRedirects bool
	noReplay    bool
}

// WithoutAuthReplay makes the service renew an expired token after a 401 but not send the refused
// request again: its 401 is returned. A request that must not be sent twice (an SMS) uses it.
func WithoutAuthReplay() ServiceOption {
	return func(o *serviceOptions) { o.noReplay = true }
}

// WithoutRedirects makes the service treat a redirect as the answer (an error) instead of following it:
// a followed 307/308 would send the same request, an SMS, a second time to another address.
func WithoutRedirects() ServiceOption {
	return func(o *serviceOptions) { o.noRedirects = true }
}

// WithRoundTripper makes the service send every request, login included, through base instead of
// http.DefaultTransport: a caller observes or redirects the traffic (a test server, a transport that
// tells whether a request was written).
func WithRoundTripper(base http.RoundTripper) ServiceOption {
	return func(o *serviceOptions) { o.base = base }
}

func NewService(
	cfg Config,
	logger *logrus.Logger,
	httpCfg *headers.Config,
	opts ...ServiceOption,
) Service {
	var o serviceOptions
	for _, opt := range opts {
		opt(&o)
	}
	httpClient := &http.Client{Timeout: apiTimeout, Transport: o.base}
	if o.noRedirects {
		httpClient.CheckRedirect = noRedirect
	}

	// Unauthenticated client used by the token refresher to obtain a token
	// without recursing back through itself.
	baseConfig := eskizapi.NewConfiguration()
	baseConfig.Servers = eskizapi.ServerConfigurations{{URL: cfg.URL()}}
	baseConfig.HTTPClient = httpClient
	refresher := &tokenRefresher{cfg: cfg, client: eskizapi.NewAPIClient(baseConfig)}

	// Create log transport for request/response logging
	logTransport := middleware.NewLogTransport(
		logger,
		httpCfg,
		true, // log request bodies
		true, // log response bodies
		"eskiz",
	)
	if o.base != nil {
		logTransport.Base = o.base
	}

	// Create authenticated client
	authClient := &http.Client{
		Timeout:   apiTimeout,
		Transport: &authRoundTripper{Base: logTransport, Refresher: refresher, NoReplay: o.noReplay},
	}
	if o.noRedirects {
		authClient.CheckRedirect = noRedirect
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

func noRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

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

	if o.DispatchID == 0 {
		o.DispatchID = time.Now().UnixMilli()
	}
	dispatchID := int(o.DispatchID)

	batchReq := eskizapi.SendSmsBatchRequest{Messages: inner, DispatchId: &dispatchID}
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
	return models.NewBatchResult(res, o.DispatchID), nil
}

func (s *service) GetMessagesByDispatch(ctx context.Context, dispatchID int64) ([]models.MessageStatus, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}
	if dispatchID <= 0 {
		return nil, ErrInvalidMessage
	}
	res, httpResp, err := s.client.DefaultApi.
		GetUserMessagesByDispatch(ctx).
		DispatchId(strconv.FormatInt(dispatchID, 10)).
		Execute()
	drain(httpResp)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, ErrNilResponse
	}
	return models.MessageStatusesFromResponse(res), nil
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
	status := models.NewSMSStatus(res)
	if status == nil {
		return nil, ErrNilResponse
	}
	return status, nil
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
	bal := models.NewBalance(res)
	if bal == nil {
		return nil, ErrNilResponse
	}
	return bal, nil
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
