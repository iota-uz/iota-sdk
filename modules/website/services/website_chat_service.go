package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/crm/services"
)

type WebsiteChatService struct {
	chatService *services.ChatService
	clientRepo  client.Repository
}

func NewWebsiteChatService(
	chatService *services.ChatService,
	clientRepo client.Repository,
) *WebsiteChatService {
	return &WebsiteChatService{
		chatService: chatService,
		clientRepo:  clientRepo,
	}
}

func (s *WebsiteChatService) CreateThread(ctx context.Context, contact string) (chat.Chat, error) {
	var member chat.Member
	email, err := internet.NewEmail(contact)
	if err == nil {
		member, err = s.memberFromEmail(ctx, email)
		if err != nil {
			return nil, err
		}
	}

	p, err := phone.NewFromE164(contact)
	if err == nil {
		member, err = s.memberFromPhone(ctx, p)
		if err != nil {
			return nil, err
		}
	}

	if member == nil {
		return nil, fmt.Errorf("invalid contact: %s", contact)
	}

	chatEntity := chat.New(
		member.Sender().(chat.ClientSender).ClientID(),
		chat.WithMembers([]chat.Member{member}),
	)

	createdChat, err := s.chatService.Save(ctx, chatEntity)
	if err != nil {
		return nil, err
	}

	return createdChat, nil
}

func (s *WebsiteChatService) memberFromPhone(ctx context.Context, phoneNumber phone.Phone) (chat.Member, error) {
	match, err := s.clientRepo.GetByContactValue(ctx, client.ContactTypePhone, phoneNumber.Value())
	if err == nil {
		var contactID uint
		for _, contact := range match.Contacts() {
			if contact.Type() == client.ContactTypePhone && contact.Value() == phoneNumber.Value() {
				contactID = contact.ID()
				break
			}
		}
		return chat.NewMember(
			chat.NewClientSender(
				chat.WebsiteTransport,
				match.ID(),
				contactID,
				match.FirstName(),
				match.LastName(),
			),
		), nil
	} else if err != nil && !errors.Is(err, persistence.ErrClientNotFound) {
		return nil, err
	}

	c, err := client.New(phoneNumber.Value(), client.WithPhone(phoneNumber))
	if err != nil {
		return nil, err
	}

	clientEntity, err := s.clientRepo.Save(ctx, c)
	if err != nil {
		return nil, err
	}
	var contactID uint
	for _, contact := range clientEntity.Contacts() {
		if contact.Type() == client.ContactTypePhone && contact.Value() == phoneNumber.Value() {
			contactID = contact.ID()
			break
		}
	}
	member := chat.NewMember(
		chat.NewClientSender(
			chat.WebsiteTransport,
			clientEntity.ID(),
			contactID,
			clientEntity.FirstName(),
			clientEntity.LastName(),
		),
	)
	return member, nil
}

func (s *WebsiteChatService) memberFromEmail(ctx context.Context, email internet.Email) (chat.Member, error) {
	match, err := s.clientRepo.GetByContactValue(ctx, client.ContactTypeEmail, email.Value())
	if err == nil {
		var contactID uint
		for _, contact := range match.Contacts() {
			if contact.Type() == client.ContactTypeEmail && contact.Value() == email.Value() {
				contactID = contact.ID()
				break
			}
		}
		return chat.NewMember(
			chat.NewClientSender(
				chat.WebsiteTransport,
				match.ID(),
				contactID,
				match.FirstName(),
				match.LastName(),
			),
		), nil
	} else if err != nil && !errors.Is(err, persistence.ErrClientNotFound) {
		return nil, err
	}

	c, err := client.New(email.Value(), client.WithEmail(email))
	if err != nil {
		return nil, err
	}

	clientEntity, err := s.clientRepo.Save(ctx, c)
	if err != nil {
		return nil, err
	}
	var contactID uint
	for _, contact := range clientEntity.Contacts() {
		if contact.Type() == client.ContactTypeEmail && contact.Value() == email.Value() {
			contactID = contact.ID()
			break
		}
	}
	member := chat.NewMember(
		chat.NewClientSender(
			chat.WebsiteTransport,
			clientEntity.ID(),
			contactID,
			clientEntity.FirstName(),
			clientEntity.LastName(),
		),
	)
	return member, nil
}
