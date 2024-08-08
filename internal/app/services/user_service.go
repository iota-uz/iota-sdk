package services

import (
	"context"

	"github.com/iota-agency/iota-erp/internal/domain/user"
	"github.com/iota-agency/iota-erp/pkg/composables"
	"github.com/iota-agency/iota-erp/sdk/event"
)

type UserService struct {
	Repo      user.Repository
	Publisher *event.Publisher
}

func NewUserService(repo user.Repository, app *Application) *UserService {
	return &UserService{
		Repo:      repo,
		Publisher: app.EventPublisher,
	}
}

func (s *UserService) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	return s.Repo.GetByEmail(ctx, email)
}

func (s *UserService) Count(ctx context.Context) (int64, error) {
	return s.Repo.Count(ctx)
}

func (s *UserService) GetAll(ctx context.Context) ([]*user.User, error) {
	return s.Repo.GetAll(ctx)
}

func (s *UserService) GetByID(ctx context.Context, id int64) (*user.User, error) {
	return s.Repo.GetByID(ctx, id)
}

func (s *UserService) GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*user.User, error) {
	return s.Repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *UserService) Create(ctx context.Context, data *user.User) error {
	ev := &user.Created{
		Data: &(*data),
	}
	if u, err := composables.UseUser(ctx); err == nil {
		ev.Sender = u
	}
	if sess, err := composables.UseSession(ctx); err == nil {
		ev.Session = sess
	}

	if err := data.SetPassword(*data.Password); err != nil {
		return err
	}
	if err := s.Repo.Create(ctx, data); err != nil {
		return err
	}
	ev.Result = &(*data)
	s.Publisher.Publish(ev)
	return nil
}

func (s *UserService) UpdateLastAction(ctx context.Context, id int64) error {
	return s.Repo.UpdateLastAction(ctx, id)
}

func (s *UserService) UpdateLastLogin(ctx context.Context, id int64) error {
	return s.Repo.UpdateLastLogin(ctx, id)
}

func (s *UserService) Update(ctx context.Context, data *user.User) error {
	evt := &user.Updated{
		Data: &(*data),
	}
	if u, err := composables.UseUser(ctx); err == nil {
		evt.Sender = u
	}
	if sess, err := composables.UseSession(ctx); err == nil {
		evt.Session = sess
	}
	if err := s.Repo.Update(ctx, data); err != nil {
		return err
	}
	evt.Result = &(*data)
	s.Publisher.Publish(evt)
	return nil
}

func (s *UserService) Delete(ctx context.Context, id int64) (*user.User, error) {
	evt := &user.Deleted{}
	if u, err := composables.UseUser(ctx); err == nil {
		evt.Sender = u
	}
	if sess, err := composables.UseSession(ctx); err == nil {
		evt.Session = sess
	}
	entity, err := s.Repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.Repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	evt.Result = entity
	s.Publisher.Publish(evt)
	return entity, nil
}
