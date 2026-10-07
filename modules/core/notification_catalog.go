package core

import (
	"context"
	"fmt"
	"strconv"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
)

const UserCreatedNotificationEvent = "core.user.created.v1"

func newCoreNotificationCatalog(extra []notifications.Definition) (*notifications.Catalog, error) {
	catalog := notifications.NewCatalog()
	definitions := append([]notifications.Definition{notifications.TestDefinition(), {
		Module: "core",
		Key:    UserCreatedNotificationEvent,
		Name: map[string]string{
			"en": "User created", "ru": "Создан пользователь", "uz": "Foydalanuvchi yaratildi",
			"uz-Cyrl": "Фойдаланувчи яратилди", "pt-BR": "Usuário criado", "zh": "用户已创建",
		},
		Description: map[string]string{
			"en":      "Notify selected users when a back-office account is created. Recipients need permission to read users.",
			"ru":      "Уведомлять выбранных получателей о создании пользователя. Получателям нужно право просмотра пользователей.",
			"uz":      "Hisob yaratilganda tanlangan oluvchilarni xabardor qilish. Oluvchilarda foydalanuvchilarni ko‘rish huquqi bo‘lishi kerak.",
			"uz-Cyrl": "Ҳисоб яратилганда танланган олувчиларни хабардор қилиш. Олувчиларда фойдаланувчиларни кўриш ҳуқуқи бўлиши керак.",
			"pt-BR":   "Notificar os destinatários quando uma conta é criada. Exige permissão para visualizar usuários.",
			"zh":      "创建账户时通知选定的收件人。收件人需要查看用户的权限。",
		},
		RequiredPermission: permissions.UserRead,
		DefaultLevel:       notification.LevelSuccess,
		PayloadFields:      []notifications.PayloadField{{Key: "user_id", Required: true}, {Key: "name", Required: true}},
		ValidatePayload: func(event notifications.Event) error {
			id, err := strconv.ParseUint(event.Data["user_id"], 10, 32)
			if err != nil || id == 0 {
				return fmt.Errorf("invalid user-created user_id")
			}
			return nil
		},
		RecipientKeys: []notifications.RecipientDefinition{
			{Key: "actor", Name: map[string]string{"en": "Event actor", "ru": "Инициатор события", "uz": "Voqea tashabbuskori", "uz-Cyrl": "Воқеа ташаббускори", "pt-BR": "Responsável pelo evento", "zh": "事件发起人"}, Resolve: func(_ context.Context, event notifications.Event) ([]uint, error) {
				if event.ActorUserID == 0 {
					return nil, nil
				}
				return []uint{event.ActorUserID}, nil
			}},
			{Key: "created_user", Name: map[string]string{"en": "Created user", "ru": "Созданный пользователь", "uz": "Yaratilgan foydalanuvchi", "uz-Cyrl": "Яратилган фойдаланувчи", "pt-BR": "Usuário criado", "zh": "新建用户"}, Resolve: func(_ context.Context, event notifications.Event) ([]uint, error) {
				id, err := strconv.ParseUint(event.Data["user_id"], 10, 32)
				if err != nil {
					return nil, err
				}
				return []uint{uint(id)}, nil
			}},
		},
		Render: func(event notifications.Event, language string) (notifications.Content, error) {
			id, err := strconv.ParseUint(event.Data["user_id"], 10, 64)
			if err != nil || id == 0 || event.Data["name"] == "" {
				return notifications.Content{}, fmt.Errorf("invalid user-created notification payload")
			}
			titles := map[string]string{"en": "User created", "ru": "Создан пользователь", "uz": "Foydalanuvchi yaratildi", "uz-Cyrl": "Фойдаланувчи яратилди", "pt-BR": "Usuário criado", "zh": "用户已创建"}
			return notifications.Content{Title: notifications.Localized(titles, language), Body: event.Data["name"], ActionURL: fmt.Sprintf("/users/%d", id)}, nil
		},
	}}, extra...)
	for _, definition := range definitions {
		if err := catalog.Register(definition); err != nil {
			return nil, err
		}
	}
	return catalog, nil
}
