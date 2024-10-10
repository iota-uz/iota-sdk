package app

import (
	"github.com/iota-agency/iota-erp/internal/app/services"
	"github.com/iota-agency/iota-erp/internal/infrastructure/persistence"
	"github.com/iota-agency/iota-erp/sdk/event"
	"gorm.io/gorm"
)

func New(db *gorm.DB) *services.Application {
	eventPublisher := event.NewEventPublisher()

	app := &services.Application{
		DD:                  db,
		EventPublisher:      eventPublisher,
		SessionService:      services.NewSessionService(persistence.NewSessionRepository(), eventPublisher),
		UploadService:       services.NewUploadService(persistence.NewUploadRepository(), eventPublisher),
		UserService:         services.NewUserService(persistence.NewUserRepository(), eventPublisher),
		RoleService:         services.NewRoleService(persistence.NewRoleRepository(), eventPublisher),
		PaymentService:      services.NewPaymentService(persistence.NewPaymentRepository(), eventPublisher),
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
	}
	app.AuthService = services.NewAuthService(app)
	app.DialogueService = services.NewDialogueService(persistence.NewDialogueRepository(), app)
	app.EmbeddingService = services.NewEmbeddingService(app)
	return app
}
