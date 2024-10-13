package app

import (
	"github.com/iota-agency/iota-erp/internal/app/handlers"
	"github.com/iota-agency/iota-erp/internal/app/services"
	"github.com/iota-agency/iota-erp/internal/infrastructure/persistence"
	"github.com/iota-agency/iota-erp/sdk/event"
	"gorm.io/gorm"
)

func New(db *gorm.DB) *services.Application {
	eventPublisher := event.NewEventPublisher()

	moneyAccountService := services.NewMoneyAccountService(
		persistence.NewMoneyAccountRepository(),
		eventPublisher,
	)
	app := &services.Application{
		DD:                  db,
		EventPublisher:      eventPublisher,
		MoneyAccountService: moneyAccountService,
		SessionService:      services.NewSessionService(persistence.NewSessionRepository(), eventPublisher),
		UploadService:       services.NewUploadService(persistence.NewUploadRepository(), eventPublisher),
		UserService:         services.NewUserService(persistence.NewUserRepository(), eventPublisher),
		RoleService:         services.NewRoleService(persistence.NewRoleRepository(), eventPublisher),
		PaymentService: services.NewPaymentService(
			persistence.NewPaymentRepository(), eventPublisher, moneyAccountService,
		),
		ProjectStageService: services.NewProjectStageService(persistence.NewProjectStageRepository(), eventPublisher),
		CurrencyService:     services.NewCurrencyService(persistence.NewCurrencyRepository(), eventPublisher),
		ExpenseCategoryService: services.NewExpenseCategoryService(
			persistence.NewExpenseCategoryRepository(),
			eventPublisher,
		),
		PositionService: services.NewPositionService(persistence.NewPositionRepository(), eventPublisher),
		EmployeeService: services.NewEmployeeService(persistence.NewEmployeeRepository(), eventPublisher),
		AuthLogService:  services.NewAuthLogService(persistence.NewAuthLogRepository(), eventPublisher),
		PromptService:   services.NewPromptService(persistence.NewPromptRepository(), eventPublisher),
		ExpenseService: services.NewExpenseService(
			persistence.NewExpenseRepository(), eventPublisher, moneyAccountService,
		),
	}

	handlers.RegisterSessionEventHandlers(db, eventPublisher, app.AuthLogService)
	app.AuthService = services.NewAuthService(app)
	app.DialogueService = services.NewDialogueService(persistence.NewDialogueRepository(), app)
	app.EmbeddingService = services.NewEmbeddingService(app)
	return app
}
