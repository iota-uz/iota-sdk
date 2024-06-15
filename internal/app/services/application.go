package services

import (
	"github.com/iota-agency/iota-erp/internal/infrastracture/event"
)

type Application struct {
	EventPublisher  *event.Publisher
	AuthService     *AuthService
	UserService     *UserService
	UploadService   *UploadService
	DialogueService *DialogueService
	PromptService   *PromptService
	SessionService  *SessionService
	AuthLogService  *AuthLogService
}
