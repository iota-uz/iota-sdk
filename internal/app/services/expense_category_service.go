package services

import (
	"context"

	"github.com/iota-agency/iota-erp/internal/domain/expense_category"
	"github.com/iota-agency/iota-erp/pkg/composables"
	"github.com/iota-agency/iota-erp/sdk/event"
)

type ExpenseCategoryService struct {
	Repo      category.Repository
	Publisher *event.Publisher
}

func NewExpenseCategoryService(repo category.Repository, app *Application) *ExpenseCategoryService {
	return &ExpenseCategoryService{
		Repo:      repo,
		Publisher: app.EventPublisher,
	}
}

func (s *ExpenseCategoryService) GetByID(ctx context.Context, id int64) (*category.ExpenseCategory, error) {
	return s.Repo.GetByID(ctx, id)
}

func (s *ExpenseCategoryService) Count(ctx context.Context) (int64, error) {
	return s.Repo.Count(ctx)
}

func (s *ExpenseCategoryService) GetAll(ctx context.Context) ([]*category.ExpenseCategory, error) {
	return s.Repo.GetAll(ctx)
}

func (s *ExpenseCategoryService) GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*category.ExpenseCategory, error) {
	return s.Repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *ExpenseCategoryService) Create(ctx context.Context, data *category.ExpenseCategory) error {
	ev := &category.Created{
		Data: &(*data),
	}
	if u, err := composables.UseUser(ctx); err == nil {
		ev.Sender = u
	}
	if sess, err := composables.UseSession(ctx); err == nil {
		ev.Session = sess
	}

	if err := s.Repo.Create(ctx, data); err != nil {
		return err
	}
	ev.Result = &(*data)
	s.Publisher.Publish(ev)
	return nil
}

func (s *ExpenseCategoryService) Update(ctx context.Context, data *category.ExpenseCategory) error {
	evt := &category.Updated{
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

func (s *ExpenseCategoryService) Delete(ctx context.Context, id int64) (*category.ExpenseCategory, error) {
	evt := &category.Deleted{}
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
