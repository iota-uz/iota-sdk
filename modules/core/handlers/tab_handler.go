package handlers

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tab"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/sirupsen/logrus"
)

type TabHandler struct {
	app        application.Application
	tabService tab.Repository
	logger     *logrus.Logger
}

func NewTabHandler(
	app application.Application,
	tabService tab.Repository,
	logger *logrus.Logger,
) *TabHandler {
	return &TabHandler{
		app:        app,
		tabService: tabService,
		logger:     logger,
	}
}

func (h *TabHandler) Register(publisher eventbus.EventBus) {
	publisher.Subscribe(h.HandleUserCreated)
}

func (h *TabHandler) HandleUserCreated(event *user.CreatedEvent) {
	h.createUserTabs(context.Background(), event.Result)
}

func (h *TabHandler) createUserTabs(ctx context.Context, user user.User) {
	if h.app == nil {
		h.logger.Error("Application not set in tab handler")
		return
	}

	items := h.app.NavItems(i18n.NewLocalizer(h.app.Bundle(), string(user.UILanguage())))
	hrefs := h.getAccessibleNavItems(items, user)

	tabs := make([]*tab.Tab, 0, len(hrefs))
	for i, href := range hrefs {
		tabs = append(tabs, &tab.Tab{
			UserID:   user.ID(),
			Href:     href,
			Position: uint(i),
		})
	}

	if len(tabs) > 0 {
		ctxWithUser := context.WithValue(ctx, constants.UserKey, user)
		if err := h.tabService.DeleteUserTabs(ctxWithUser, user.ID()); err != nil {
			h.logger.Errorf("Failed to delete existing tabs for user %d: %v", user.ID(), err)
			return
		}
		if err := h.tabService.CreateMany(ctxWithUser, tabs); err != nil {
			h.logger.Errorf("Failed to create tab for user %d: %v", user.ID(), err)
		}
	}
}

func (h *TabHandler) getAccessibleNavItems(items []types.NavigationItem, user user.User) []string {
	var result []string

	for _, item := range items {
		if item.HasPermission(user) {
			if item.Href != "" {
				result = append(result, item.Href)
			}

			if len(item.Children) > 0 {
				childItems := h.getAccessibleNavItems(item.Children, user)
				result = append(result, childItems...)
			}
		}
	}

	return result
}
