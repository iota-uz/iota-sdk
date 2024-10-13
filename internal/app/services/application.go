package services

import (
	"github.com/iota-agency/iota-erp/sdk/event"
	"gorm.io/gorm"
)

type Application struct {
	DD                     *gorm.DB
	EventPublisher         event.Publisher
	AuthService            *AuthService
	UserService            *UserService
	UploadService          *UploadService
	DialogueService        *DialogueService
	PromptService          *PromptService
	SessionService         *SessionService
	AuthLogService         *AuthLogService
	EmbeddingService       *EmbeddingService
	EmployeeService        *EmployeeService
	PaymentService         *PaymentService
	RoleService            *RoleService
	PositionService        *PositionService
	ExpenseCategoryService *ExpenseCategoryService
	ProjectStageService    *ProjectStageService
	CurrencyService        *CurrencyService
	MoneyAccountService    *MoneyAccountService
	ExpenseService         *ExpenseService
}
