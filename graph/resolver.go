package graph

import (
	"github.com/iota-agency/iota-erp/pkg/authentication"
	"gorm.io/gorm"
)

//go:generate go run github.com/99designs/gqlgen generate

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	Db          *gorm.DB
	AuthService *authentication.Service
}
