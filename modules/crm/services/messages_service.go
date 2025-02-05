package services

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/phone"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/entities/message"
	cpassproviders "github.com/iota-uz/iota-sdk/modules/crm/infrastructure/cpass-providers"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type SendMessageDTO struct {
	ChatID      uint
	Message     string
	Attachments []*upload.Upload
}

type MessagesService struct {
	repo          message.Repository
	clientRepo    client.Repository
	cpassProvider cpassproviders.Provider
	chatService   *ChatService
	publisher     eventbus.EventBus
}

func NewMessagesService(
	repo message.Repository,
	clientRepo client.Repository,
	cpassProvider cpassproviders.Provider,
	chatService *ChatService,
	publisher eventbus.EventBus,
) *MessagesService {
	return &MessagesService{
		repo:          repo,
		clientRepo:    clientRepo,
		cpassProvider: cpassProvider,
		chatService:   chatService,
		publisher:     publisher,
	}
}

func (s *MessagesService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *MessagesService) GetAll(ctx context.Context) ([]message.Message, error) {
	return s.repo.GetAll(ctx)
}

func (s *MessagesService) GetByID(ctx context.Context, id uint) (message.Message, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *MessagesService) GetPaginated(ctx context.Context, params *message.FindParams) ([]message.Message, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *MessagesService) GetByChatID(ctx context.Context, chatID uint) ([]message.Message, error) {
	return s.repo.GetByChatID(ctx, chatID)
}

func (s *MessagesService) RegisterClientMessage(
	ctx context.Context,
	params *cpassproviders.ReceivedMessageEvent,
) error {
	p, err := phone.NewFromE164(params.From)
	if err != nil {
		return err
	}
	clientEntity, err := s.clientRepo.GetByPhone(ctx, p.Value())
	if err != nil {
		return err
	}

	chatEntity, err := s.chatService.GetByClientIDOrCreate(ctx, clientEntity.ID())
	if err != nil {
		return err
	}

	createdMessage, err := s.repo.Create(ctx, message.New(
		chatEntity.ID(),
		params.Body,
		message.NewClientSender(clientEntity.ID(), clientEntity.FirstName(), clientEntity.LastName()),
	))

	if err != nil {
		return err
	}

	ev, err := message.NewCreatedEvent(ctx, createdMessage)
	if err != nil {
		return err
	}

	s.publisher.Publish(ev)
	return nil
}

func (s *MessagesService) SendMessage(ctx context.Context, dto SendMessageDTO) (message.Message, error) {
	user, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}
	chatEntity, err := s.chatService.GetByID(ctx, dto.ChatID)
	if err != nil {
		return nil, err
	}
	createdMessage, err := s.repo.Create(ctx, message.New(
		dto.ChatID,
		dto.Message,
		message.NewUserSender(user.ID(), user.FirstName(), user.LastName()),
	))
	if err != nil {
		return nil, err
	}

	if err := s.cpassProvider.SendMessage(ctx, cpassproviders.SendMessageDTO{
		From:    configuration.Use().TwilioPhoneNumber,
		To:      chatEntity.Client().Phone().Value(),
		Message: dto.Message,
	}); err != nil {
		return nil, err
	}
	ev, err := message.NewCreatedEvent(ctx, createdMessage)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(ev)
	return createdMessage, nil
}

func (s *MessagesService) Delete(ctx context.Context, id uint) (message.Message, error) {
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	ev, err := message.NewDeletedEvent(ctx, entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(ev)
	return entity, nil
}
