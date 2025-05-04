package mappers

import (
	"fmt"
	"strconv"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/passport"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	messagetemplate "github.com/iota-uz/iota-sdk/modules/crm/domain/entities/message-template"
	"github.com/iota-uz/iota-sdk/modules/crm/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

func ClientToViewModel(entity client.Client) *viewmodels.Client {
	var dateOfBirth string
	if entity.DateOfBirth() != nil {
		dateOfBirth = entity.DateOfBirth().Format(time.RFC3339)
	}

	var passport viewmodels.Passport
	if entity.Passport() != nil {
		passport = PassportToViewModel(entity.Passport())
	}

	var pin string
	if entity.Pin() != nil {
		pin = entity.Pin().Value()
	}

	var gender string
	if entity.Gender() != nil {
		gender = entity.Gender().String()
	}

	var email string
	if entity.Email() != nil {
		email = entity.Email().Value()
	}

	var phone string
	if entity.Phone() != nil {
		phone = entity.Phone().Value()
	}

	return &viewmodels.Client{
		ID:          strconv.FormatUint(uint64(entity.ID()), 10),
		FirstName:   entity.FirstName(),
		LastName:    entity.LastName(),
		MiddleName:  entity.MiddleName(),
		Phone:       phone,
		Email:       email,
		DateOfBirth: dateOfBirth,
		Address:     entity.Address(),
		Gender:      gender,
		Pin:         pin,
		CountryCode: "", // Phone doesn't have CountryCode method
		Passport:    passport,
		Comments:    entity.Comments(),
		CreatedAt:   entity.CreatedAt().Format(time.RFC3339),
		UpdatedAt:   entity.UpdatedAt().Format(time.RFC3339),
	}
}

func PassportToViewModel(p passport.Passport) viewmodels.Passport {
	return viewmodels.Passport{
		ID:             p.ID().String(),
		Series:         p.Series(),
		Number:         p.Number(),
		FirstName:      p.FirstName(),
		LastName:       p.LastName(),
		MiddleName:     p.MiddleName(),
		Gender:         p.Gender().String(),
		BirthDate:      p.BirthDate().Format(time.RFC3339),
		BirthPlace:     p.BirthPlace(),
		Nationality:    p.Nationality(),
		PassportType:   p.PassportType(),
		IssuedAt:       p.IssuedAt().Format(time.RFC3339),
		IssuedBy:       p.IssuedBy(),
		IssuingCountry: p.IssuingCountry(),
		ExpiresAt:      p.ExpiresAt().Format(time.RFC3339),
	}
}

func SenderToViewModel(entity chat.Sender) viewmodels.MessageSender {
	if v, ok := entity.(chat.ClientSender); ok {
		return viewmodels.NewClientMessageSender(
			strconv.FormatUint(uint64(v.ClientID()), 10),
			shared.GetInitials(v.FirstName(), v.LastName()),
		)
	}
	if v, ok := entity.(chat.UserSender); ok {
		return viewmodels.NewUserMessageSender(
			strconv.FormatUint(uint64(v.UserID()), 10),
			shared.GetInitials(v.FirstName(), v.LastName()),
		)
	}
	panic(fmt.Sprintf("unknown sender type: %T", entity))
}

func MemberToViewModel(entity chat.Member) *viewmodels.Member {
	return &viewmodels.Member{
		ID:        entity.ID().String(),
		Transport: string(entity.Transport()),
		Sender:    SenderToViewModel(entity.Sender()),
		CreatedAt: entity.CreatedAt().Format(time.RFC3339),
	}
}

func MessageToViewModel(entity chat.Message) *viewmodels.Message {
	return &viewmodels.Message{
		ID:        strconv.FormatUint(uint64(entity.ID()), 10),
		Sender:    SenderToViewModel(entity.Sender().Sender()),
		Message:   entity.Message(),
		CreatedAt: entity.CreatedAt(),
	}
}

func ChatToViewModel(entity chat.Chat, clientEntity client.Client) *viewmodels.Chat {
	return &viewmodels.Chat{
		ID:             strconv.FormatUint(uint64(entity.ID()), 10),
		Client:         ClientToViewModel(clientEntity),
		Messages:       mapping.MapViewModels(entity.Messages(), MessageToViewModel),
		UnreadMessages: entity.UnreadMessages(),
		CreatedAt:      entity.CreatedAt().Format(time.RFC3339),
	}
}

func MessageTemplateToViewModel(entity messagetemplate.MessageTemplate) *viewmodels.MessageTemplate {
	return &viewmodels.MessageTemplate{
		ID:        strconv.FormatUint(uint64(entity.ID()), 10),
		Template:  entity.Template(),
		CreatedAt: entity.CreatedAt().Format(time.RFC3339),
	}
}
