package main

import (
	"github.com/iota-agency/iota-erp/models"
	"github.com/iota-agency/iota-erp/pkg/utils"
	"github.com/iota-agency/iota-erp/sdk/mapper"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
)

func createInitialUser(tx *gorm.DB, email, password string) error {
	var count int64
	if err := tx.Model(&models.User{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	role := &models.Role{
		Name:        "admin",
		Description: mapper.Pointer("Administrator"),
	}
	if err := tx.Save(role).Error; err != nil {
		return err
	}
	user := &models.User{
		FirstName: "Admin",
		LastName:  "User",
		Email:     email,
		Password:  &password,
	}
	if err := user.SetPassword(password); err != nil {
		return err
	}
	if err := tx.Save(user).Error; err != nil {
		return err
	}
	userRole := &models.UserRole{
		UserId: user.Id,
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
			return createInitialUser(tx, userEmail, userPassword)
		}); err != nil {
			panic(err)
		}
	}
}
