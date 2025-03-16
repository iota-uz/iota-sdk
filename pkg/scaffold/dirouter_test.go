package scaffold

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

// Mock application for testing
type mockApp struct {
	application.Application
}

func (m *mockApp) Bundle() *i18n.Bundle {
	bundle := i18n.NewBundle(language.English)
	return bundle
}

// Create a test context with necessary dependencies
func setupTestContext() context.Context {
	ctx := context.Background()

	// Add user to context
	email, _ := internet.NewEmail("john.doe@example.com")
	u := user.New("John", "Doe", email, "en")
	ctx = composables.WithUser(ctx, u)

	// Add localizer to context
	bundle := i18n.NewBundle(language.English)
	localizer := i18n.NewLocalizer(bundle, "en")
	ctx = composables.WithLocalizer(ctx, localizer)

	// Add page context
	pageCtx := &types.PageContext{}
	ctx = composables.WithPageCtx(ctx, pageCtx)

	// Add app to context
	app := &mockApp{}
	ctx = context.WithValue(ctx, constants.AppKey, app)

	return ctx
}

func diTestHandler(
	r *http.Request,
	w http.ResponseWriter,
	localizer *i18n.Localizer,
	u user.User,
) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	message := localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: "NavigationLinks.Dashboard",
	})

	// Write response
	w.Write([]byte(fmt.Sprintf("NavigationLinks.Dashboard: %s", message)))
	w.Write([]byte("\n"))
	w.Write([]byte(fmt.Sprintf("Fullname: %s %s", u.FirstName(), u.LastName())))
	w.Write([]byte("\n"))
	w.Write([]byte(fmt.Sprintf("ID: %d", id)))
	w.Write([]byte("\n"))
}

// Handler without DI approach - manually fetches dependencies
func NonDIHandler(w http.ResponseWriter, r *http.Request) {
	// Manually extract dependencies from context
	localizer, ok := composables.UseLocalizer(r.Context())
	if !ok {
		http.Error(w, "localizer not found", http.StatusInternalServerError)
		return
	}

	u, err := composables.UseUser(r.Context())
	if err != nil {
		http.Error(w, "user not found", http.StatusInternalServerError)
		return
	}

	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	message := localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: "NavigationLinks.Dashboard",
	})

	// Write response
	w.Write([]byte(fmt.Sprintf("NavigationLinks.Dashboard: %s", message)))
	w.Write([]byte("\n"))
	w.Write([]byte(fmt.Sprintf("Fullname: %s %s", u.FirstName(), u.LastName())))
	w.Write([]byte("\n"))
	w.Write([]byte(fmt.Sprintf("ID: %d", id)))
	w.Write([]byte("\n"))
}

func BenchmarkDIRouter(b *testing.B) {
	ctx := setupTestContext()
	req, _ := http.NewRequest("GET", "/123", nil)
	req = req.WithContext(ctx)

	handler := &DIHandler{
		value: reflect.ValueOf(diTestHandler),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		handler.Handler()(rr, req)
	}
}

func BenchmarkNonDIRouter(b *testing.B) {
	ctx := setupTestContext()
	req, _ := http.NewRequest("GET", "/123", nil)
	req = req.WithContext(ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		NonDIHandler(rr, req)
	}
}

