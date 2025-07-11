package eskiz

import (
	"context"
	eskizapi "github.com/iota-uz/eskiz"
	"github.com/iota-uz/iota-sdk/pkg/eskiz/models"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/sirupsen/logrus"
	"net/http"
	"time"
)

const (
	logRequestBody  = true
	logResponseBody = true
	apiTimeout      = 30 * time.Second
)

type Service interface {
	SendSMS(ctx context.Context, model models.SendSMS) (models.SendSMSResult, error)
}

func NewService(
	cfg Config,
	logger *logrus.Logger,
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

	// Create authenticated client
	authClient := &http.Client{
		Timeout: apiTimeout,
		Transport: &authRoundTripper{
			Base: middleware.NewLogTransport(
				logger,
				logRequestBody,
				logResponseBody,
			),
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
	req := s.client.DefaultApi.
		SendSms(ctx).
		MobilePhone(model.PhoneNumber()).
		Message(model.Message())

	if model.From() != "" {
		req = req.From(model.From())
	}
	if model.CallbackUrl() != "" {
		req = req.CallbackUrl(model.CallbackUrl())
	}

	res, _, err := req.Execute()
	if err != nil {
		return nil, err
	}

	return models.NewSendSMSResult(res), nil
}
