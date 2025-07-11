package eskiz

import (
	"context"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/eskiz/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_RealAPI(t *testing.T) {
	t.Skip("Real API test - requires valid credentials")

	url := "https://notify.eskiz.uz"
	email := "your-email@example.com"
	password := "your-password"

	cfg := NewConfig(url, email, password)
	service := NewService(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sms, err := models.NewSendSMS("998901234567", "Test message from integration test")
	require.NoError(t, err)

	result, err := service.SendSMS(ctx, sms)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.NotEmpty(t, result.Id())
	assert.NotEmpty(t, result.Message())
	assert.NotEmpty(t, result.Status())

	t.Logf("SMS sent successfully: ID=%s, Message=%s, Status=%s",
		result.Id(), result.Message(), result.Status())
}
