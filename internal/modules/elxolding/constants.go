package elxolding

import (
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/role"
	"github.com/iota-agency/iota-erp/internal/domain/entities/permission"
	"time"
)

var (
	CEO = role.Role{
		ID:          1,
		Name:        "Руко́водитель",
		Description: "Руко́водитель",
		Permissions: permission.Permissions,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	Admin = role.Role{
		ID:          2,
		Name:        "Администратор",
		Description: "Администратор",
		Permissions: permission.Permissions,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	Printing = role.Role{
		ID:          3,
		Name:        "Полиграфия",
		Description: "Полиграфия",
		Permissions: permission.Permissions,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	QualityAssurance = role.Role{
		ID:          4,
		Name:        "ОТК",
		Description: "Отдел технического контроля",
		Permissions: permission.Permissions,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	WarehouseEmployee = role.Role{
		ID:          5,
		Name:        "Склад",
		Description: "Склад",
		Permissions: permission.Permissions,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
)

var (
	Roles = []role.Role{
		CEO,
		Admin,
		Printing,
		QualityAssurance,
		WarehouseEmployee,
	}
)
