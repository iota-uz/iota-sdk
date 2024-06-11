package app

import (
	"github.com/iota-agency/iota-erp/internal/domain/auth"
	"github.com/iota-agency/iota-erp/internal/domain/upload"
	"github.com/iota-agency/iota-erp/internal/domain/user"
	infrastructure "github.com/iota-agency/iota-erp/internal/infrastracture"
	"github.com/iota-agency/iota-erp/internal/infrastracture/event"
)

type Application struct {
	AuthService   *auth.Service
	UserService   *user.Service
	UploadService *upload.Service
}

func New(registry *infrastructure.RepositoryRegistry, eventPublisher *event.Publisher) *Application {
	authService := auth.NewService()
	userService := user.NewUserService(registry.GetUserRepository(), eventPublisher)
	uploadService := upload.NewService(registry.GetUploadRepository(), eventPublisher)
	return &Application{
		AuthService:   authService,
		UserService:   userService,
		UploadService: uploadService,
	}
}
