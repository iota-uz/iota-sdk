package app

import (
	"github.com/iota-agency/iota-erp/internal/app/services"
	"github.com/iota-agency/iota-erp/internal/infrastracture/event"
	"github.com/iota-agency/iota-erp/internal/infrastracture/persistence"
)

func New() *services.Application {
	eventPublisher := event.NewEventPublisher()
	userRepository := persistence.NewUserRepository()
	uploadRepository := persistence.NewUploadRepository()
	dialogueRepository := persistence.NewDialogueRepository()
	promptRepository := persistence.NewPromptRepository()
	sessionRepository := persistence.NewSessionRepository()
	authLogRepository := persistence.NewAuthLogRepository()

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

	app.AuthService = authService
	app.UserService = userService
	app.UploadService = uploadService
	app.DialogueService = dialogueService
	app.PromptService = promptService
	app.SessionService = sessionService
	app.AuthLogService = authLogService
	return app
}
