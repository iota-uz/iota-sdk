package services

import (
	"context"

	stage "github.com/iota-agency/iota-erp/internal/domain/entities/project_stages"
	"github.com/iota-agency/iota-erp/pkg/composables"
	"github.com/iota-agency/iota-erp/sdk/event"
)

type ProjectStageService struct {
	repo      stage.Repository
	publisher *event.Publisher
}

func NewProjectStageService(repo stage.Repository, app *Application) *ProjectStageService {
	return &ProjectStageService{
		repo:      repo,
		publisher: app.EventPublisher,
	}
}

func (s *ProjectStageService) Count(ctx context.Context) (uint, error) {
	return s.repo.Count(ctx)
}

func (s *ProjectStageService) GetAll(ctx context.Context) ([]*stage.ProjectStage, error) {
	return s.repo.GetAll(ctx)
}

func (s *ProjectStageService) GetByID(ctx context.Context, id uint) (*stage.ProjectStage, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ProjectStageService) GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*stage.ProjectStage, error) {
	return s.repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *ProjectStageService) Create(ctx context.Context, data *stage.CreateDTO) error {
	ev := &stage.Created{
		Data: &(*data),
	}
	if u, err := composables.UseUser(ctx); err == nil {
		ev.Sender = u
	}
	if sess, err := composables.UseSession(ctx); err == nil {
		ev.Session = sess
	}
	entity := data.ToEntity()
	if err := s.repo.Create(ctx, entity); err != nil {
		return err
	}
	ev.Result = entity
	s.publisher.Publish(ev)
	return nil
}

func (s *ProjectStageService) Update(ctx context.Context, id uint, data *stage.UpdateDTO) error {
	ev := &stage.Updated{
		Data: &(*data),
	}
	if u, err := composables.UseUser(ctx); err == nil {
		ev.Sender = u
	}
	if sess, err := composables.UseSession(ctx); err == nil {
		ev.Session = sess
	}
	entity := data.ToEntity(id)
	if err := s.repo.Update(ctx, entity); err != nil {
		return err
	}
	ev.Result = entity
	s.publisher.Publish(ev)
	return nil
}

func (s *ProjectStageService) Delete(ctx context.Context, id uint) (*stage.ProjectStage, error) {
	ev := &stage.Deleted{}
	if u, err := composables.UseUser(ctx); err == nil {
		ev.Sender = u
	}
	if sess, err := composables.UseSession(ctx); err == nil {
		ev.Session = sess
	}
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	ev.Result = entity
	s.publisher.Publish(ev)
	return entity, nil
}
