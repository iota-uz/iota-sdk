package services

import (
	"context"

	messagetemplate "github.com/iota-uz/iota-sdk/modules/crm/domain/entities/message-template"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type MessageTemplateService struct {
	repo      messagetemplate.Repository
	publisher eventbus.EventBus
}

func NewMessageTemplateService(
	repo messagetemplate.Repository, publisher eventbus.EventBus,
) *MessageTemplateService {
	return &MessageTemplateService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *MessageTemplateService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *MessageTemplateService) GetAll(ctx context.Context) ([]messagetemplate.MessageTemplate, error) {
	return s.repo.GetAll(ctx)
}

func (s *MessageTemplateService) GetByID(ctx context.Context, id uint) (messagetemplate.MessageTemplate, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *MessageTemplateService) GetPaginated(
	ctx context.Context,
	params *messagetemplate.FindParams,
) ([]messagetemplate.MessageTemplate, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *MessageTemplateService) Create(
	ctx context.Context,
	data *messagetemplate.CreateDTO,
) (messagetemplate.MessageTemplate, error) {
	entity := data.ToEntity()
	createdEntity, err := s.repo.Create(ctx, entity)
	if err != nil {
		return nil, err
	}
	return createdEntity, nil
}

func (s *MessageTemplateService) Update(
	ctx context.Context,
	id uint,
	data *messagetemplate.UpdateDTO,
) (messagetemplate.MessageTemplate, error) {
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	updatedEntity, err := s.repo.Update(ctx, data.Apply(entity))
	if err != nil {
		return nil, err
	}
	return updatedEntity, nil
}

func (s *MessageTemplateService) Delete(ctx context.Context, id uint) (messagetemplate.MessageTemplate, error) {
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	return entity, nil
}
