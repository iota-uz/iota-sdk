// Package services provides this package.
package services

import (
	"context"
	"strconv"

	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/listext"
)

type ClientService struct {
	repo          client.Repository
	publisher     eventbus.EventBus
	historyWriter history.Writer
}

func NewClientService(
	repo client.Repository,
	publisher eventbus.EventBus,
	historyWriter history.Writer,
) *ClientService {
	return &ClientService{
		repo:          repo,
		publisher:     publisher,
		historyWriter: historyWriter,
	}
}

func (s *ClientService) Count(ctx context.Context, params *client.FindParams) (int64, error) {
	return s.repo.Count(ctx, params)
}

func (s *ClientService) GetPaginated(ctx context.Context, params *client.FindParams) ([]client.Client, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *ClientService) GetByID(ctx context.Context, id uint) (client.Client, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ClientService) GetByPhone(ctx context.Context, phoneNumber string) (client.Client, error) {
	return s.repo.GetByPhone(ctx, phoneNumber)
}

func (s *ClientService) Create(ctx context.Context, data client.Client) error {
	var createdClient client.Client
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		created, err := s.create(txCtx, data)
		if err != nil {
			return err
		}
		createdClient = created
		return nil
	})
	if err != nil {
		return err
	}
	createdEvent, err := client.NewCreatedEvent(ctx, data)
	if err != nil {
		return err
	}
	createdEvent.Result = createdClient
	s.publisher.Publish(createdEvent)
	return nil
}

func (s *ClientService) create(txCtx context.Context, data client.Client) (client.Client, error) {
	created, err := s.repo.Save(txCtx, data)
	if err != nil {
		return nil, err
	}
	if err := history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
		Type: ClientCreated, Version: 1, Resource: ClientResource, ID: strconv.FormatUint(uint64(created.ID()), 10),
		After: clientSnapshot(created),
	}); err != nil {
		return nil, err
	}
	return created, nil
}

func (s *ClientService) Update(ctx context.Context, data client.Client) error {
	var updatedClient client.Client
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		before, err := s.repo.GetByID(txCtx, data.ID())
		if err != nil {
			return err
		}
		updated, err := s.repo.Save(txCtx, data)
		if err != nil {
			return err
		}
		updatedClient = updated
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: ClientUpdated, Version: 1, Resource: ClientResource, ID: strconv.FormatUint(uint64(data.ID()), 10),
			Before: clientSnapshot(before), After: clientSnapshot(updated),
		})
	})
	if err != nil {
		return err
	}
	updatedEvent, err := client.NewUpdatedEvent(ctx, data)
	if err != nil {
		return err
	}
	updatedEvent.Result = updatedClient
	s.publisher.Publish(updatedEvent)
	return nil
}

func (s *ClientService) Delete(ctx context.Context, id uint) (client.Client, error) {
	var entity client.Client
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
			Type: ClientDeleted, Version: 1, Resource: ClientResource, ID: strconv.FormatUint(uint64(id), 10),
			Before: clientSnapshot(entity),
		})
	})
	if err != nil {
		return nil, err
	}
	deletedEvent, err := client.NewDeletedEvent(ctx, entity)
	if err != nil {
		return nil, err
	}
	deletedEvent.Result = entity
	s.publisher.Publish(deletedEvent)
	return entity, nil
}

// ExtensionView reads the columns extension packages contribute to the clients
// list for the given clients. It returns nil when the repository does not
// support extension columns or none applies to the current user.
func (s *ClientService) ExtensionView(ctx context.Context, params *client.FindParams, purpose listext.Purpose, ids []uint) (*listext.View, error) {
	reader, ok := s.repo.(client.ExtensionReader)
	if !ok {
		return nil, nil
	}
	return reader.ExtensionView(ctx, params, purpose, ids)
}
