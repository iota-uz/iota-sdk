// Package productservice provides this package.
package productservice

import (
	"context"
	"errors"

	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type ProductService struct {
	repo          product.Repository
	publisher     eventbus.EventBus
	historyWriter history.Writer
}

func NewProductService(
	repo product.Repository,
	publisher eventbus.EventBus,
	historyWriter history.Writer,
) *ProductService {
	return &ProductService{
		repo:          repo,
		publisher:     publisher,
		historyWriter: historyWriter,
	}
}

func (s *ProductService) Count(ctx context.Context, params *product.CountParams) (int64, error) {
	if err := composables.CanUser(ctx, permissions.ProductRead); err != nil {
		return 0, err
	}
	return s.repo.Count(ctx, params)
}

func (s *ProductService) CountInStock(ctx context.Context, params *product.CountParams) (int64, error) {
	if err := composables.CanUser(ctx, permissions.ProductRead); err != nil {
		return 0, err
	}
	return s.repo.Count(ctx, params)
}

func (s *ProductService) GetByID(ctx context.Context, id uint) (product.Product, error) {
	if err := composables.CanUser(ctx, permissions.ProductRead); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *ProductService) GetAll(ctx context.Context) ([]product.Product, error) {
	if err := composables.CanUser(ctx, permissions.ProductRead); err != nil {
		return nil, err
	}
	return s.repo.GetAll(ctx)
}

func (s *ProductService) GetPaginated(
	ctx context.Context, params *product.FindParams,
) ([]product.Product, error) {
	if err := composables.CanUser(ctx, permissions.ProductRead); err != nil {
		return nil, err
	}
	return s.repo.GetPaginated(ctx, params)
}

func (s *ProductService) Create(ctx context.Context, data *product.CreateDTO) error {
	if err := composables.CanUser(ctx, permissions.ProductCreate); err != nil {
		return err
	}
	existing, err := s.repo.GetByRfid(ctx, data.Rfid)
	if existing != nil {
		return NewErrDuplicateRfid(data.Rfid)
	} else if err != nil && !errors.Is(err, persistence.ErrProductNotFound) {
		return err
	}
	entity, err := data.ToEntity()
	if err != nil {
		return err
	}
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		entity, err = s.repo.Create(txCtx, entity)
		if err != nil {
			return err
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: ProductCreated, Version: 1, Resource: ProductResource, ID: productID(entity),
			After: productSnapshot(entity),
		})
	})
	if err != nil {
		return err
	}
	createdEvent, err := product.NewCreatedEvent(ctx, *data, entity)
	if err != nil {
		return err
	}
	s.publisher.Publish(createdEvent)
	return nil
}

func (s *ProductService) CreateProductsFromTags(
	ctx context.Context, data *product.CreateProductsFromTagsDTO,
) ([]product.Product, error) {
	if err := composables.CanUser(ctx, permissions.ProductCreate); err != nil {
		return nil, err
	}
	dtos := make([]*product.CreateDTO, len(data.Tags))
	for i, tag := range data.Tags {
		dtos[i] = &product.CreateDTO{
			Rfid:       tag,
			PositionID: data.PositionID,
			Status:     string(product.InStock),
		}
	}
	entities, err := s.BulkCreate(ctx, dtos)
	if err != nil {
		return nil, err
	}
	return entities, nil
}

func (s *ProductService) ValidateProducts(ctx context.Context, tags []string) ([]product.Product, []product.Product, error) {
	if err := composables.CanUser(ctx, permissions.ProductUpdate); err != nil {
		return nil, nil, err
	}
	var valid []product.Product
	var invalid []product.Product
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		entities, err := s.repo.GetByRfidMany(txCtx, tags)
		if err != nil {
			return err
		}
		for _, entity := range entities {
			before := entity
			if entity.Status() == product.InDevelopment {
				valid = append(valid, entity)
				entity = entity.SetStatus(product.Approved)
			} else {
				invalid = append(invalid, entity)
			}
			if err := s.repo.Update(txCtx, entity); err != nil {
				return err
			}
			if err := history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
				Type: ProductUpdated, Version: 1, Resource: ProductResource, ID: productID(entity),
				Before: productSnapshot(before), After: productSnapshot(entity),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return valid, invalid, nil
}

func (s *ProductService) BulkCreate(ctx context.Context, data []*product.CreateDTO) ([]product.Product, error) {
	if err := composables.CanUser(ctx, permissions.ProductCreate); err != nil {
		return nil, err
	}
	entities := make([]product.Product, len(data))
	for i, d := range data {
		entity, err := d.ToEntity()
		if err != nil {
			return nil, err
		}
		entities[i] = entity
	}
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		created, err := s.repo.BulkCreate(txCtx, entities)
		if err != nil {
			return err
		}
		for _, entity := range created {
			if err := history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
				Type: ProductCreated, Version: 1, Resource: ProductResource, ID: productID(entity),
				After: productSnapshot(entity),
			}); err != nil {
				return err
			}
		}
		entities = created
		return nil
	})
	if err != nil {
		return nil, err
	}
	for i, d := range data {
		createdEvent, err := product.NewCreatedEvent(ctx, *d, entities[i])
		if err != nil {
			return nil, err
		}
		s.publisher.Publish(createdEvent)
	}
	return entities, nil
}

func (s *ProductService) Update(ctx context.Context, id uint, data *product.UpdateDTO) error {
	if err := composables.CanUser(ctx, permissions.ProductUpdate); err != nil {
		return err
	}
	existing, err := s.repo.GetByRfid(ctx, data.Rfid)
	if existing != nil && existing.ID() != id {
		return NewErrDuplicateRfid(data.Rfid)
	} else if err != nil && !errors.Is(err, persistence.ErrProductNotFound) {
		return err
	}
	entity, err := data.ToEntity(id)
	if err != nil {
		return err
	}
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		before, err := s.repo.GetByID(txCtx, id)
		if err != nil {
			return err
		}
		if err := s.repo.Update(txCtx, entity); err != nil {
			return err
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: ProductUpdated, Version: 1, Resource: ProductResource, ID: productID(entity),
			Before: productSnapshot(before), After: productSnapshot(entity),
		})
	})
	if err != nil {
		return err
	}
	updatedEvent, err := product.NewUpdatedEvent(ctx, *data, entity)
	if err != nil {
		return err
	}
	s.publisher.Publish(updatedEvent)
	return nil
}

func (s *ProductService) Delete(ctx context.Context, id uint) (product.Product, error) {
	if err := composables.CanUser(ctx, permissions.ProductDelete); err != nil {
		return nil, err
	}
	var entity product.Product
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		entity, err = s.repo.GetByID(txCtx, id)
		if err != nil {
			return err
		}
		if err := s.repo.Delete(txCtx, id); err != nil {
			return err
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: ProductDeleted, Version: 1, Resource: ProductResource, ID: productID(entity),
			Before: productSnapshot(entity),
		})
	})
	if err != nil {
		return nil, err
	}
	deletedEvent, err := product.NewDeletedEvent(ctx, entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(deletedEvent)
	return entity, nil
}
