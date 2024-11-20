package services

import (
	"context"
	product2 "github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-agency/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/event"
)

type ProductService struct {
	repo            product2.Repository
	publisher       event.Publisher
	positionService *PositionService
}

func NewProductService(
	repo product2.Repository,
	publisher event.Publisher,
	positionService *PositionService,
) *ProductService {
	return &ProductService{
		repo:            repo,
		publisher:       publisher,
		positionService: positionService,
	}
}

func (s *ProductService) Count(ctx context.Context) (int64, error) {
	if err := composables.CanUser(ctx, permissions.ProductRead); err != nil {
		return 0, err
	}
	return s.repo.Count(ctx)
}

func (s *ProductService) GetByID(ctx context.Context, id uint) (*product2.Product, error) {
	if err := composables.CanUser(ctx, permissions.ProductRead); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *ProductService) GetAll(ctx context.Context) ([]*product2.Product, error) {
	if err := composables.CanUser(ctx, permissions.ProductRead); err != nil {
		return nil, err
	}
	return s.repo.GetAll(ctx)
}

func (s *ProductService) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*product2.Product, error) {
	if err := composables.CanUser(ctx, permissions.ProductRead); err != nil {
		return nil, err
	}
	return s.repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *ProductService) Create(ctx context.Context, data *product2.CreateDTO) error {
	if err := composables.CanUser(ctx, permissions.ProductCreate); err != nil {
		return err
	}
	entity, err := data.ToEntity()
	if err != nil {
		return err
	}
	if err := s.repo.Create(ctx, entity); err != nil {
		return err
	}
	createdEvent, err := product2.NewCreatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	s.publisher.Publish(createdEvent)
	return nil
}

func (s *ProductService) Update(ctx context.Context, id uint, data *product2.UpdateDTO) error {
	if err := composables.CanUser(ctx, permissions.ProductUpdate); err != nil {
		return err
	}
	entity, err := data.ToEntity(id)
	if err != nil {
		return err
	}
	if err := s.repo.Update(ctx, entity); err != nil {
		return err
	}
	updatedEvent, err := product2.NewUpdatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	s.publisher.Publish(updatedEvent)
	return nil
}

func (s *ProductService) Delete(ctx context.Context, id uint) (*product2.Product, error) {
	if err := composables.CanUser(ctx, permissions.ProductDelete); err != nil {
		return nil, err
	}
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	deletedEvent, err := product2.NewDeletedEvent(ctx, *entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(deletedEvent)
	return entity, nil
}
