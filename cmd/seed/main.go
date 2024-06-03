package main

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/domain/user"
	"github.com/iota-agency/iota-erp/internal/infrastracture/persistence"
	"github.com/iota-agency/iota-erp/models"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/iota-agency/iota-erp/sdk/mapper"
	"github.com/iota-agency/iota-erp/sdk/utils"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
)

func createInitialUser(ctx context.Context, email, password string) error {
	userRepo := persistence.NewUserRepository()
	count, err := userRepo.Count(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	tx, _ := composables.UseTx(ctx)
	role := &models.Role{
		Name:        "admin",
		Description: mapper.Pointer("Administrator"),
	}
	if err := tx.Save(role).Error; err != nil {
		return err
	}
	u := &user.User{
		FirstName: "Admin",
		LastName:  "User",
		Email:     email,
		Password:  &password,
	}
	if err := u.SetPassword(password); err != nil {
		return err
	}
	if err := userRepo.Update(ctx, u); err != nil {
		return err
	}
	userRole := &models.UserRole{
		UserId: u.Id,
		RoleId: role.Id,
	}
	return tx.Save(userRole).Error
}

func main() {
	utils.LoadEnv()
	log.Println("Connecting to database:", utils.DbOpts())
	db, err := gorm.Open(postgres.Open(utils.DbOpts()), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	userEmail := utils.GetEnv("INITIAL_USER_EMAIL", "")
	userPassword := utils.GetEnv("INITIAL_USER_PASSWORD", "")
	if userEmail != "" && userPassword != "" {
		if err := db.Transaction(func(tx *gorm.DB) error {
			ctx := composables.WithTx(context.Background(), tx)
			return createInitialUser(ctx, userEmail, userPassword)
		}); err != nil {
			panic(err)
		}
	}
}
