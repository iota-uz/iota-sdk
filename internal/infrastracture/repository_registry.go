package infrastructure

import (
	"github.com/iota-agency/iota-erp/internal/domain/upload"
	"github.com/iota-agency/iota-erp/internal/domain/user"
)

type RepositoryRegistry struct {
	userRepository    user.Repository
	uploadsRepository upload.Repository
}

func NewRepositoryRegistry() *RepositoryRegistry {
	return &RepositoryRegistry{}
}

func (r *RepositoryRegistry) RegisterUserRepository(repo user.Repository) {
	r.userRepository = repo
}

func (r *RepositoryRegistry) GetUserRepository() user.Repository {
	if r.userRepository == nil {
		panic("UserRepository is not registered")
	}
	return r.userRepository
}

func (r *RepositoryRegistry) RegisterUploadRepository(repo upload.Repository) {
	r.uploadsRepository = repo
}

func (r *RepositoryRegistry) GetUploadRepository() upload.Repository {
	if r.uploadsRepository == nil {
		panic("UploadsRepository is not registered")
	}
	return r.uploadsRepository
}
