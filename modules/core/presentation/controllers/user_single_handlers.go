package controllers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/a-h/templ"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/components/base/slot"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/users"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

func (c *UsersController) GetSingle(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	userService *services.UserService,
	groupQueryService *services.GroupQueryService,
	sessionService *services.SessionService,
) {
	ctx := r.Context()
	pageCtx := composables.UsePageCtx(ctx)

	id, err := shared.ParseID(r)
	if err != nil {
		logger.WithError(err).Error("error parsing user id")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	us, err := userService.GetByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("error retrieving user")
		http.Error(w, "Error retrieving user", http.StatusInternalServerError)
		return
	}

	userViewModel := mappers.UserToViewModel(us)
	slots := slot.NewManager()

	if userViewModel.IsBlocked {
		vm := *userViewModel
		slots.Async(
			users.SingleSlotBlocked,
			func(ctx context.Context) (templ.Component, error) {
				if vm.BlockedBy != "" && vm.BlockedBy != "0" {
					blockedByID, err := strconv.ParseUint(vm.BlockedBy, 10, 64)
					if err == nil {
						blocker, err := userService.GetByID(ctx, uint(blockedByID))
						if err == nil {
							vm.BlockedByUser = mappers.UserToViewModel(blocker).Title()
						}
					}
				}

				return users.BlockedBanner(&vm), nil
			},
			slot.WithSlotSourceFallback(templ.Raw(pageCtx.T("Common.Loading"))),
		)
	}

	if composables.CanUser(ctx, permissions.SessionRead) == nil {
		currentUser, _ := composables.UseUser(ctx)
		canDelete := currentUser != nil && currentUser.Can(permissions.SessionDelete)
		targetID := id
		targetVM := userViewModel

		slots.Async(
			users.SingleSlotSessions,
			func(ctx context.Context) (templ.Component, error) {
				sessionList, err := sessionService.GetByUserID(ctx, targetID)
				if err != nil {
					return nil, err
				}

				return users.SessionsInfo(buildSessionsTable(ctx, targetVM.ID, sessionList, canDelete)), nil
			},
			slot.WithSlotSourceFallback(templ.Raw(pageCtx.T("Common.Loading"))),
		)
	}

	slots.Async(
		users.SingleSlotGroups,
		func(ctx context.Context) (templ.Component, error) {
			if len(userViewModel.GroupIDs) == 0 {
				return escapedText(""), nil
			}

			groups, _, err := groupQueryService.FindGroups(ctx, &query.GroupFindParams{
				Limit:  len(userViewModel.GroupIDs),
				Offset: 0,
				SortBy: query.SortBy{
					Fields: []repo.SortByField[query.Field]{
						{Field: query.GroupFieldName, Ascending: true},
					},
				},
				Filters: []query.GroupFilter{
					{
						Column: query.GroupFieldID,
						Filter: repo.In(userViewModel.GroupIDs),
					},
				},
			})
			if err != nil {
				return nil, err
			}

			out := make([]string, 0, len(groups))
			for _, group := range groups {
				out = append(out, group.Name)
			}

			return escapedText(strings.Join(out, ", ")), nil
		},
		slot.WithSlotSourceFallback(templ.Raw(pageCtx.T("Common.Loading"))),
	)

	if c.configureSingleSlots != nil {
		c.configureSingleSlots(ctx, us, slots)
	}

	templ.Handler(users.Single(&users.SingleProps{
		User:                     userViewModel,
		Slots:                    slots,
		ResourcePermissionGroups: FilterCheckedResourcePermissionGroups(c.resourcePermissionGroups(us.Permissions()...)),
	}), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *UsersController) GetEdit(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	userService *services.UserService,
	roleService *services.RoleService,
	groupQueryService *services.GroupQueryService,
) {
	id, err := shared.ParseID(r)
	if err != nil {
		logger.WithError(err).Error("error parsing user id")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	props, err := c.buildEditFormProps(r.Context(), logger, userService, roleService, groupQueryService, id, nil)
	if err != nil {
		logger.WithError(err).Error("error building edit form props")
		http.Error(w, "Error retrieving user information", http.StatusInternalServerError)
		return
	}

	templ.Handler(users.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *UsersController) GetBlockDrawer(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	userService *services.UserService,
) {
	if !htmx.IsHxRequest(r) {
		http.Error(w, "Expected HTMX request", http.StatusBadRequest)
		return
	}

	if err := composables.CanUser(r.Context(), permissions.UserUpdateBlockStatus); err != nil {
		logger.WithError(err).Error("error lacks permission")
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	id, err := shared.ParseID(r)
	if err != nil {
		logger.WithError(err).Error("error parsing user id")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	targetUser, err := userService.GetByID(r.Context(), id)
	if err != nil {
		logger.WithError(err).Error("error retrieving user")
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if err := renderBlockDrawer(r.Context(), w, mappers.UserToViewModel(targetUser), map[string]string{}); err != nil {
		logger.WithError(err).Error("error rendering block drawer")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (c *UsersController) BlockUser(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	userService *services.UserService,
	roleService *services.RoleService,
	groupQueryService *services.GroupQueryService,
) {
	if !htmx.IsHxRequest(r) {
		http.Error(w, "Expected HTMX request", http.StatusBadRequest)
		return
	}

	if err := composables.CanUser(r.Context(), permissions.UserUpdateBlockStatus); err != nil {
		logger.WithError(err).Error("error lacks permission")
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	pageCtx := composables.UsePageCtx(r.Context())
	id, err := shared.ParseID(r)
	if err != nil {
		logger.WithError(err).Error("error parsing user id")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		logger.WithError(err).Error("error parsing form")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	blockReason := strings.TrimSpace(r.FormValue("BlockReason"))
	blockReasonLength := utf8.RuneCountInString(blockReason)
	errors := map[string]string{}

	switch {
	case blockReason == "":
		errors["BlockReason"] = pageCtx.T("Users.Block.Errors.ReasonRequired")
	case blockReasonLength < 3:
		errors["BlockReason"] = pageCtx.T("Users.Block.Errors.ReasonTooShort")
	case blockReasonLength > 1024:
		errors["BlockReason"] = pageCtx.T("Users.Block.Errors.ReasonTooLong")
	}

	if len(errors) > 0 {
		targetUser, err := userService.GetByID(r.Context(), id)
		if err != nil {
			logger.WithError(err).Error("error fetching user")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := renderBlockDrawer(r.Context(), w, mappers.UserToViewModel(targetUser), errors); err != nil {
			logger.WithError(err).Error("error rendering block drawer")
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	if _, err := userService.BlockUser(r.Context(), id, blockReason); err != nil {
		logger.WithError(err).Error("error blocking user")
		errors["BlockReason"] = pageCtx.T("Users.Block.Errors.OperationFailed")

		targetUser, fetchErr := userService.GetByID(r.Context(), id)
		if fetchErr != nil {
			logger.WithError(fetchErr).Error("error fetching user")
			http.Error(w, fetchErr.Error(), http.StatusInternalServerError)
			return
		}

		if err := renderBlockDrawer(r.Context(), w, mappers.UserToViewModel(targetUser), errors); err != nil {
			logger.WithError(err).Error("error rendering block drawer")
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	logger.
		WithField("blocked_by", composables.MustUseUser(r.Context()).ID()).
		WithField("target_id", id).
		WithField("action", "block").
		Info("user blocked")

	props, err := c.buildEditFormProps(r.Context(), logger, userService, roleService, groupQueryService, id, nil)
	if err != nil {
		logger.WithError(err).Error("error building edit form props")
		http.Error(w, "Error retrieving user information", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if _, err := io.WriteString(&buf, fmt.Sprintf(`<div id="block-user-drawer-%d"></div>`, id)); err != nil {
		logger.WithError(err).Error("error writing block drawer placeholder")
		http.Error(w, "Error rendering response", http.StatusInternalServerError)
		return
	}

	if err := renderEditContentOOB(r.Context(), &buf, props); err != nil {
		logger.WithError(err).Error("error buffering edit content")
		http.Error(w, "Error rendering response", http.StatusInternalServerError)
		return
	}

	if _, err := buf.WriteTo(w); err != nil {
		logger.WithError(err).Error("error writing response")
	}
}

func (c *UsersController) UnblockUser(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	userService *services.UserService,
	roleService *services.RoleService,
	groupQueryService *services.GroupQueryService,
) {
	if !htmx.IsHxRequest(r) {
		http.Error(w, "Expected HTMX request", http.StatusBadRequest)
		return
	}

	if err := composables.CanUser(r.Context(), permissions.UserUpdateBlockStatus); err != nil {
		logger.WithError(err).Error("error lacks permission")
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	id, err := shared.ParseID(r)
	if err != nil {
		logger.WithError(err).Error("error parsing user id")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if _, err := userService.UnblockUser(r.Context(), id); err != nil {
		logger.WithError(err).Error("error unblocking user")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	logger.
		WithField("unblocked_by", composables.MustUseUser(r.Context()).ID()).
		WithField("target_id", id).
		WithField("action", "unblock").
		Info("user unblocked")

	props, err := c.buildEditFormProps(r.Context(), logger, userService, roleService, groupQueryService, id, nil)
	if err != nil {
		logger.WithError(err).Error("error building edit form props")
		http.Error(w, "Error retrieving user information", http.StatusInternalServerError)
		return
	}

	if err := users.EditForm(props).Render(r.Context(), w); err != nil {
		logger.WithError(err).Error("error rendering edit form")
		http.Error(w, "Error rendering response", http.StatusInternalServerError)
	}
}
