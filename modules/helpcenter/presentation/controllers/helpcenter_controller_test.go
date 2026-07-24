package controllers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/helpcenter/services"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func TestHelpCenterController_Media(t *testing.T) {
	controller := newMediaTestController(t)
	req := httptest.NewRequest(http.MethodGet, "/help/media/images/lifecycle.svg", nil)
	ctx := intl.WithLocale(context.Background(), language.Russian)
	req = mux.SetURLVars(req.WithContext(ctx), map[string]string{"path": "images/lifecycle.svg"})
	rec := httptest.NewRecorder()

	controller.media(rec, req)

	resp := rec.Result()
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "image/svg+xml", resp.Header.Get("Content-Type"))
	require.Equal(t, "private, max-age=3600", resp.Header.Get("Cache-Control"))
	require.Equal(t, "default-src 'none'; style-src 'unsafe-inline'; sandbox", resp.Header.Get("Content-Security-Policy"))
	require.Equal(t, "same-origin", resp.Header.Get("Cross-Origin-Resource-Policy"))
	require.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	require.Equal(t, "<svg>fallback</svg>", rec.Body.String())
}

func TestHelpCenterController_MediaNotFoundAndInvalidPaths(t *testing.T) {
	controller := newMediaTestController(t)

	for _, mediaPath := range []string{"images/missing.png", "../secret.png"} {
		t.Run(mediaPath, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/help/media/"+mediaPath, nil)
			req = mux.SetURLVars(req, map[string]string{"path": mediaPath})
			rec := httptest.NewRecorder()

			controller.media(rec, req)

			require.Equal(t, http.StatusNotFound, rec.Code)
		})
	}
}

func newMediaTestController(t *testing.T) *HelpCenterController {
	t.Helper()
	contentFS := fstest.MapFS{
		"content/en/images/lifecycle.svg": &fstest.MapFile{Data: []byte("<svg>fallback</svg>")},
	}
	return &HelpCenterController{
		contentService: services.NewContentService(services.ContentConfig{
			FS:            contentFS,
			Root:          "content",
			Locales:       []string{"en", "ru"},
			DefaultLocale: "en",
		}),
	}
}
