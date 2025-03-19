package services

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tab"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type UserService struct {
	repo       user.Repository
	publisher  eventbus.EventBus
	app        application.Application
	tabService *TabService
}

func NewUserService(repo user.Repository, publisher eventbus.EventBus) *UserService {
	return &UserService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *UserService) SetTabService(tabService *TabService) {
	s.tabService = tabService
}

func (s *UserService) SetApplication(app application.Application) {
	s.app = app
}

func (s *UserService) getAccessibleNavItems(items []types.NavigationItem, user user.User) []string {
	var result []string

	for _, item := range items {
		if item.HasPermission(user) {
			if item.Href != "" {
				result = append(result, item.Href)
			}

			if len(item.Children) > 0 {
				childItems := s.getAccessibleNavItems(item.Children, user)
				result = append(result, childItems...)
			}
		}
	}

	return result
}

func (s *UserService) createUserTabs(ctx context.Context, user user.User) error {
	if s.app == nil || s.tabService == nil {
		return nil
	}

	items := s.app.NavItems(i18n.NewLocalizer(s.app.Bundle(), string(user.UILanguage())))
	hrefs := s.getAccessibleNavItems(items, user)

	tabs := make([]*tab.CreateDTO, 0, len(hrefs))
	for i, href := range hrefs {
		tabs = append(tabs, &tab.CreateDTO{
			Href:     href,
			UserID:   user.ID(),
			Position: uint(i),
		})
	}

	if len(tabs) > 0 {
		ctxWithUser := context.WithValue(ctx, constants.UserKey, user)
		_, err := s.tabService.CreateManyUserTabs(ctxWithUser, user.ID(), tabs)
		return err
	}
	return nil
}

func (s *UserService) GetByEmail(ctx context.Context, email string) (user.User, error) {
	return s.repo.GetByEmail(ctx, email)
}

func (s *UserService) Count(ctx context.Context, params *user.FindParams) (int64, error) {
	return s.repo.Count(ctx, params)
}

func (s *UserService) GetAll(ctx context.Context) ([]user.User, error) {
	return s.repo.GetAll(ctx)
}

func (s *UserService) GetByID(ctx context.Context, id uint) (user.User, error) {
	if err := composables.CanUser(ctx, permissions.UserRead); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *UserService) GetPaginated(ctx context.Context, params *user.FindParams) ([]user.User, error) {
	if err := composables.CanUser(ctx, permissions.UserRead); err != nil {
		return nil, err
	}
	return s.repo.GetPaginated(ctx, params)
}

func (s *UserService) GetPaginatedWithTotal(ctx context.Context, params *user.FindParams) ([]user.User, int64, error) {
	if err := composables.CanUser(ctx, permissions.UserRead); err != nil {
		return nil, 0, err
	}
	us, err := s.repo.GetPaginated(ctx, params)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.Count(ctx, params)
	if err != nil {
		return nil, 0, err
	}
	return us, total, nil
}

func (s *UserService) Create(ctx context.Context, data user.User) error {
	if err := composables.CanUser(ctx, permissions.UserCreate); err != nil {
		return err
	}
	tx, err := composables.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	createdEvent, err := user.NewCreatedEvent(ctx, data)
	if err != nil {
		return err
	}
	data, err = data.SetPassword(data.Password())
	if err != nil {
		return err
	}
	created, err := s.repo.Create(ctx, data)
	if err != nil {
		return err
	}

	if err := s.createUserTabs(ctx, created); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	createdEvent.Result = created
	s.publisher.Publish(createdEvent)
	return nil
}

func (s *UserService) UpdateLastAction(ctx context.Context, id uint) error {
	return s.repo.UpdateLastAction(ctx, id)
}

func (s *UserService) UpdateLastLogin(ctx context.Context, id uint) error {
	return s.repo.UpdateLastLogin(ctx, id)
}

func (s *UserService) Update(ctx context.Context, data user.User) error {
	if err := composables.CanUser(ctx, permissions.UserUpdate); err != nil {
		return err
	}
	tx, err := composables.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	updatedEvent, err := user.NewUpdatedEvent(ctx, data)
	if err != nil {
		return err
	}
	if data.Password() != "" {
		data, err = data.SetPassword(data.Password())
		if err != nil {
			return err
		}
	}
	if err := s.repo.Update(ctx, data); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	updatedEvent.Result = data
	s.publisher.Publish(updatedEvent)
	return nil
}

func (s *UserService) Delete(ctx context.Context, id uint) (user.User, error) {
	if err := composables.CanUser(ctx, permissions.UserDelete); err != nil {
		return nil, err
	}
	tx, err := composables.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	deletedEvent, err := user.NewDeletedEvent(ctx)
	if err != nil {
		return nil, err
	}
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	deletedEvent.Result = entity
	s.publisher.Publish(deletedEvent)
	return entity, nil
}
