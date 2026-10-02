package middleware

import (
	"bytes"
	"encoding/json"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/headers"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoggerRecoverSafeSingleEvent(t *testing.T) {
	for _, abort := range []bool{false, true} {
		t.Run(map[bool]string{false: "panic", true: "abort"}[abort], func(t *testing.T) {
			var output bytes.Buffer
			logger := logrus.New()
			logger.SetLevel(logrus.ErrorLevel)
			logger.SetOutput(&output)
			logger.SetFormatter(&logrus.JSONFormatter{})
			handler := WithLogger(logger, NewLoggerOptions(false, false, 0), &headers.Config{RequestID: "X-Request-ID"})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				if abort {
					panic(http.ErrAbortHandler)
				}
				panic("private-panic-secret")
			}))
			r := httptest.NewRequest(http.MethodGet, "/test?secret=private-query-secret", nil)
			r.Header.Set("X-Request-ID", "correlation-123")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			require.NotContains(t, output.String(), "private-panic-secret")
			require.NotContains(t, output.String(), "private-query-secret")
			if abort {
				require.Empty(t, output.String())
				return
			}
			require.Equal(t, 500, w.Code)
			var entry map[string]any
			decoder := json.NewDecoder(&output)
			require.NoError(t, decoder.Decode(&entry))
			require.False(t, decoder.More())
			require.Equal(t, "internal", entry["error.code"])
			require.Equal(t, "middleware.Recover", entry["error.op"])
			require.Equal(t, "correlation-123", entry["request_id"])
		})
	}
}
