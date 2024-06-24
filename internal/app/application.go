package app

import (
	"github.com/iota-agency/iota-erp/internal/app/services"
	"github.com/iota-agency/iota-erp/internal/infrastracture/persistence"
	"github.com/iota-agency/iota-erp/sdk/event"
	"gorm.io/gorm"
)

func New(db *gorm.DB) *services.Application {
	eventPublisher := event.NewEventPublisher()
	userRepository := persistence.NewUserRepository()
	uploadRepository := persistence.NewUploadRepository()
	dialogueRepository := persistence.NewDialogueRepository()
	promptRepository := persistence.NewPromptRepository()
	sessionRepository := persistence.NewSessionRepository()
	authLogRepository := persistence.NewAuthLogRepository()
	employeeRepository := persistence.NewEmployeeRepository()
	roleRepository := persistence.NewRoleRepository()
	positionRepository := persistence.NewPositionRepository()

	app := &services.Application{
		EventPublisher: eventPublisher,
	}
	authService := services.NewAuthService(app)
	userService := services.NewUserService(userRepository, app)
	uploadService := services.NewUploadService(uploadRepository, app)
	dialogueService := services.NewDialogueService(dialogueRepository, app)
	promptService := services.NewPromptService(promptRepository, app)
	sessionService := services.NewSessionService(sessionRepository, app)
	authLogService := services.NewAuthLogService(authLogRepository, app)
	employeeService := services.NewEmployeeService(employeeRepository, app)
	embeddingService := services.NewEmbeddingService(app)
	roleService := services.NewRoleService(roleRepository, app)
	positionService := services.NewPositionService(positionRepository, app)

	app.AuthService = authService
	app.UserService = userService
	app.UploadService = uploadService
	app.DialogueService = dialogueService
	app.PromptService = promptService
	app.SessionService = sessionService
	app.AuthLogService = authLogService
	app.EmbeddingService = embeddingService
	app.EmployeeService = employeeService
	app.RoleService = roleService
	app.PositionService = positionService
	return app
}
