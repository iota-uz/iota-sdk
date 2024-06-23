package services

import (
	"github.com/iota-agency/iota-erp/internal/infrastracture/event"
	"gorm.io/gorm"
)

type Application struct {
	Db               *gorm.DB
	EventPublisher   *event.Publisher
	AuthService      *AuthService
	UserService      *UserService
	UploadService    *UploadService
	DialogueService  *DialogueService
	PromptService    *PromptService
	SessionService   *SessionService
	AuthLogService   *AuthLogService
	EmbeddingService *EmbeddingService
	EmployeeService  *EmployeeService
	RoleService      *RoleService
	PositionService  *PositionService
}
