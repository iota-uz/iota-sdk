package controllers

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	sfui "github.com/iota-uz/iota-sdk/components/scaffold/table"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/users"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

func escapedText(text string) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_, err := io.WriteString(w, templ.EscapeString(text))
		return err
	})
}

func buildSessionsTable(ctx context.Context, userID string, sessions []session.Session, canDelete bool) *sfui.TableConfig {
	pageCtx := composables.UsePageCtx(ctx)
	cols := []sfui.TableColumn{
		sfui.Column("device", pageCtx.T("Users.Sessions.Device")),
		sfui.Column("browser", pageCtx.T("Users.Sessions.Browser")),
		sfui.Column("os", pageCtx.T("Users.Sessions.OS")),
		sfui.Column("ip", pageCtx.T("Users.Sessions.IP")),
		sfui.Column("createdAt", pageCtx.T("Users.Sessions.CreatedAt")),
	}
	if canDelete {
		cols = append(cols, sfui.Column("actions", pageCtx.T("Users.Sessions.Actions")))
	}

	tcfg := sfui.NewTableConfig(
		pageCtx.T("Users.Sessions.Title"),
		fmt.Sprintf("/users/%s/sessions", userID),
		sfui.WithoutSearch(),
		sfui.WithConfigurable(false),
	)
	tcfg.AddCols(cols...)

	for _, sess := range sessions {
		vm := viewmodels.SessionToViewModel(sess, "")
		cells := []sfui.TableCell{
			sfui.Cell(users.SessionDeviceCell(vm), vm.Device),
			sfui.Cell(escapedText(vm.Browser), vm.Browser),
			sfui.Cell(escapedText(vm.OS), vm.OS),
			sfui.Cell(escapedText(vm.IPAddress), vm.IPAddress),
			sfui.Cell(escapedText(vm.CreatedAt), vm.CreatedAt),
		}
		if canDelete {
			cells = append(cells, sfui.Cell(
				users.RevokeSessionButton(userID, vm.TokenID),
				nil,
			))
		}
		tcfg.AddRows(sfui.Row(cells...).ApplyOpts(
			sfui.WithRowAttrs(templ.Attributes{"id": fmt.Sprintf("session-%s", vm.TokenID)}),
		))
	}

	return tcfg
}

func (c *UsersController) GetUserSessions(
	r *http.Request,
	w http.ResponseWriter,
	u user.User,
	userService *services.UserService,
	sessionService *services.SessionService,
	logger *logrus.Entry,
) {
	if !htmx.IsHxRequest(r) {
		http.Error(w, "Expected HTMX request", http.StatusBadRequest)
		return
	}

	if !u.Can(permissions.SessionRead) {
		logger.Error("user does not have SessionRead permission")
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	userID, err := shared.ParseID(r)
	if err != nil {
		logger.WithError(err).Error("invalid user ID")
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	targetUser, err := userService.GetByID(r.Context(), userID)
	if err != nil {
		logger.WithError(err).Error("failed to fetch user")
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	sessions, err := sessionService.GetByUserID(r.Context(), userID)
	if err != nil {
		logger.WithError(err).Error("failed to fetch sessions")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	targetUserVM := mappers.UserToViewModel(targetUser)
	tcfg := buildSessionsTable(r.Context(), targetUserVM.ID, sessions, u.Can(permissions.SessionDelete))
	templ.Handler(sfui.EmbeddedContent(tcfg), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *UsersController) RevokeUserSession(
	r *http.Request,
	w http.ResponseWriter,
	u user.User,
	sessionService *services.SessionService,
	logger *logrus.Entry,
) {
	if !htmx.IsHxRequest(r) {
		http.Error(w, "Expected HTMX request", http.StatusBadRequest)
		return
	}

	if !u.Can(permissions.SessionDelete) {
		logger.Error("user does not have SessionDelete permission")
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	userID, err := shared.ParseID(r)
	if err != nil {
		logger.WithError(err).Error("invalid user ID")
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	tokenID := mux.Vars(r)["token"]
	if tokenID == "" {
		logger.Error("token is required")
		http.Error(w, "Token required", http.StatusBadRequest)
		return
	}

	sessions, err := sessionService.GetByUserID(r.Context(), userID)
	if err != nil {
		logger.WithError(err).Error("failed to fetch user sessions")
		http.Error(w, "Failed to fetch sessions", http.StatusInternalServerError)
		return
	}

	var sessionToRevoke string
	for _, s := range sessions {
		sessionVM := viewmodels.SessionToViewModel(s, "")
		if sessionVM.TokenID == tokenID {
			sessionToRevoke = s.Token()
			break
		}
	}

	if sessionToRevoke == "" {
		logger.Error("session not found")
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	if err := sessionService.TerminateSession(r.Context(), sessionToRevoke); err != nil {
		logger.WithError(err).Error("failed to terminate session")
		http.Error(w, "Failed to revoke session", http.StatusInternalServerError)
		return
	}

	pageCtx := composables.UsePageCtx(r.Context())
	logger.WithField("tokenID", tokenID).Info("session revoked successfully")
	htmx.SetTrigger(
		w,
		"showToast",
		fmt.Sprintf(
			`{"type": "success", "message": "%s"}`,
			templ.EscapeString(pageCtx.T("Users.Sessions.RevokeSuccess")),
		),
	)
	w.WriteHeader(http.StatusOK)
}
