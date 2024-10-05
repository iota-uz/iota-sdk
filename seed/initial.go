package seed

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/domain/entities/role"
	"github.com/iota-agency/iota-erp/internal/domain/entities/user"
	"github.com/iota-agency/iota-erp/internal/infrastracture/persistence"
	"github.com/iota-agency/iota-erp/sdk/composables"
)

func CreateInitialUser(ctx context.Context) error {
	userRepository := persistence.NewUserRepository()
	roleRepository := persistence.NewRoleRepository()
	tx, _ := composables.UseTx(ctx)
	r := &role.Role{
		Id:          1,
		Name:        "admin",
		Description: "Administrator",
	}
	if err := roleRepository.CreateOrUpdate(ctx, r); err != nil {
		return err
	}
	u := &user.User{
		Id:        1,
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
		UserId: u.Id,
		RoleId: r.Id,
	}
	return tx.Save(userRole).Error
}
