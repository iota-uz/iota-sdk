package app

import (
	"github.com/iota-agency/iota-erp/internal/app/services"
	"github.com/iota-agency/iota-erp/internal/infrastructure/persistence"
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
	expenseCategoriesRepository := persistence.NewExpenseCategoryRepository()
	paymentRepository := persistence.NewPaymentRepository()
	stageRepository := persistence.NewProjectStageRepository()
	currencyRepository := persistence.NewCurrencyRepository()

	app := &services.Application{
		DD:             db,
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
	expenseCategoriesService := services.NewExpenseCategoryService(expenseCategoriesRepository, app)
	paymentService := services.NewPaymentService(paymentRepository, app)
	stageService := services.NewProjectStageService(stageRepository, app)
	currencyService := services.NewCurrencyService(currencyRepository, app)

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
	app.ExpenseCategoryService = expenseCategoriesService
	app.PaymentService = paymentService
	app.ProjectStageService = stageService
	app.CurrencyService = currencyService
	return app
}
