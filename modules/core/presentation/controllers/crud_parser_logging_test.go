package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCrudMalformedInputsKeepPlainTextAndBoundedLog(t *testing.T) {
	for _, tc := range []struct{ name, body, id string }{
		{name: "malformed form", body: "amount=%"},
		{name: "malformed number", body: "amount=private-input-secret"},
		{name: "malformed ID", id: "private-input-secret"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			logger := logrus.New()
			logger.SetOutput(&output)
			logger.SetFormatter(&logrus.JSONFormatter{})
			ctx := context.WithValue(context.Background(), constants.LoggerKey, logger.WithFields(logrus.Fields{"request-id": "request-123", "private": "private-context-secret"}))
			ctx = intl.WithLocalizer(ctx, i18n.NewLocalizer(i18n.NewBundle(language.English), "en"))
			key := crud.NewIntField("id", crud.WithKey())
			controller := &CrudController[any]{primaryKeyField: key, schema: crud.NewSchema[any]("example", crud.NewFields([]crud.Field{key, crud.NewFloatField("amount")}), nil)}
			r := httptest.NewRequest(http.MethodPost, "/example", strings.NewReader(tc.body)).WithContext(ctx)
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.Header.Set("HX-Request", "true")
			w := httptest.NewRecorder()
			if tc.id != "" {
				controller.Update(w, mux.SetURLVars(r, map[string]string{"id": tc.id}))
			} else {
				controller.Create(w, r)
			}
			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Contains(t, w.Header().Get("Content-Type"), "text/plain")
			require.NotContains(t, w.Body.String(), "private-input-secret")
			require.NotContains(t, output.String(), "private-input-secret")
			require.NotContains(t, output.String(), "private-context-secret")
			var event map[string]any
			decoder := json.NewDecoder(&output)
			require.NoError(t, decoder.Decode(&event))
			require.False(t, decoder.More())
			require.Equal(t, "invalid", event["error.code"])
			require.Equal(t, "request-123", event["request_id"])
			require.Equal(t, "info", event["level"])
		})
	}
}
