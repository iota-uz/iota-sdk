package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence"
)

type SendMessageToThreadDTO struct {
	ChatID  uint
	Message string
}

type ReplyToThreadDTO struct {
	ChatID  uint
	UserID  uint
	Message string
}

type WebsiteChatService struct {
	userRepo   user.Repository
	clientRepo client.Repository
	chatRepo   chat.Repository
}

func NewWebsiteChatService(
	userRepo user.Repository,
	clientRepo client.Repository,
	chatRepo chat.Repository,
) *WebsiteChatService {
	return &WebsiteChatService{
		userRepo:   userRepo,
		clientRepo: clientRepo,
		chatRepo:   chatRepo,
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

	createdChat, err := s.chatRepo.Save(ctx, chatEntity)
	if err != nil {
		return nil, err
	}

	return createdChat, nil
}

func (s *WebsiteChatService) SendMessageToThread(ctx context.Context, dto SendMessageToThreadDTO) (chat.Chat, error) {
	chatEntity, err := s.chatRepo.GetByID(ctx, dto.ChatID)
	if err != nil {
		return nil, err
	}

	if dto.Message == "" {
		return nil, chat.ErrEmptyMessage
	}

	var member chat.Member

	for _, m := range chatEntity.Members() {
		if m.Sender().Transport() != chat.WebsiteTransport {
			continue
		}
		if v, ok := m.Sender().(chat.ClientSender); ok && v.ClientID() == chatEntity.ClientID() {
			member = m
			break
		}
	}

	if member == nil {
		return nil, fmt.Errorf("no client member found in chat")
	}

	chatEntity = chatEntity.AddMessage(chat.NewMessage(
		dto.Message,
		member,
	))
	if err != nil {
		return nil, err
	}

	savedChat, err := s.chatRepo.Save(ctx, chatEntity)
	if err != nil {
		return nil, err
	}

	return savedChat, nil
}

func (s *WebsiteChatService) ReplyToThread(
	ctx context.Context,
	dto ReplyToThreadDTO,
) (chat.Chat, error) {
	chatEntity, err := s.chatRepo.GetByID(ctx, dto.ChatID)
	if err != nil {
		return nil, err
	}

	if dto.Message == "" {
		return nil, chat.ErrEmptyMessage
	}

	var member chat.Member

	for _, m := range chatEntity.Members() {
		if m.Sender().Transport() != chat.WebsiteTransport {
			continue
		}

		if v, ok := m.Sender().(chat.UserSender); ok && v.UserID() == dto.UserID {
			member = m
			break
		}
	}

	if member == nil {
		member, err = s.memberFromUserID(ctx, dto.UserID)
		if err != nil {
			return nil, err
		}
	}

	chatEntity = chatEntity.AddMessage(chat.NewMessage(
		dto.Message,
		member,
	))
	if err != nil {
		return nil, err
	}

	savedChat, err := s.chatRepo.Save(ctx, chatEntity)
	if err != nil {
		return nil, err
	}

	return savedChat, nil
}

func (s *WebsiteChatService) findMember(ctx context.Context, contact string) (chat.Member, error) {
	email, err := internet.NewEmail(contact)
	if err == nil {
		return s.chatRepo.GetMemberByContact(ctx, string(client.ContactTypeEmail), email.Value())
	}

	p, err := phone.NewFromE164(contact)
	if err == nil {
		return s.chatRepo.GetMemberByContact(ctx, string(client.ContactTypePhone), p.Value())
	}

	return nil, fmt.Errorf("invalid contact: %s", contact)
}

func (s *WebsiteChatService) memberFromUserID(ctx context.Context, userID uint) (chat.Member, error) {
	usr, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return chat.NewMember(
		chat.NewUserSender(
			chat.WebsiteTransport,
			usr.ID(),
			usr.FirstName(),
			usr.LastName(),
		),
	), nil
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
