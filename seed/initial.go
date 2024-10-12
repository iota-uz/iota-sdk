package seed

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/role"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/user"
	"time"

	"github.com/iota-agency/iota-erp/internal/infrastructure/persistence"
	"github.com/iota-agency/iota-erp/sdk/composables"
)

func CreateInitialUser(ctx context.Context) error {
	userRepository := persistence.NewUserRepository()
	roleRepository := persistence.NewRoleRepository()
	tx, _ := composables.UseTx(ctx)
	r := &role.Role{
		ID:          1,
		Name:        "admin",
		Description: "Administrator",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := roleRepository.CreateOrUpdate(ctx, r); err != nil {
		return err
	}
	u := &user.User{
		//nolint:exhaustruct
		ID:        1,
		FirstName: "Admin",
		LastName:  "User",
		Email:     "test@gmail.com",
	}
	if err := u.SetPassword("TestPass123!"); err != nil {
		return err
	}
	if err := userRepository.CreateOrUpdate(ctx, u); err != nil {
		return err
	}
	userRole := &role.UserRole{
		UserID: u.ID,
		RoleID: r.ID,
	}
	return tx.Save(userRole).Error
}
