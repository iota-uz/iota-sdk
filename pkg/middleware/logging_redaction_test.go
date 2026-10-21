package middleware

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatFormValues_RedactsSecrets(t *testing.T) {
	t.Parallel()
	got := formatFormValues(url.Values{
		"Email":              {"user@example.com"},
		"Password":           {"Temporary-1"},
		"NewPassword":        {"my own passphrase"},
		"gorilla.csrf.Token": {"csrf"},
	})
	assert.Equal(t, "user@example.com", got["Email"])
	assert.Equal(t, redactedValue, got["Password"])
	assert.Equal(t, redactedValue, got["NewPassword"])
	assert.Equal(t, redactedValue, got["gorilla.csrf.Token"])
}

func TestRedactJSONSecrets(t *testing.T) {
	t.Parallel()
	body := map[string]interface{}{
		"password": "Temporary-1",
		"user":     map[string]interface{}{"email": "user@example.com", "newPassword": "x"},
		"items":    []interface{}{map[string]interface{}{"access_token": "abc"}},
	}
	redactJSONSecrets(body)
	assert.Equal(t, redactedValue, body["password"])
	assert.Equal(t, "user@example.com", body["user"].(map[string]interface{})["email"])
	assert.Equal(t, redactedValue, body["user"].(map[string]interface{})["newPassword"])
	assert.Equal(t, redactedValue, body["items"].([]interface{})[0].(map[string]interface{})["access_token"])
}
