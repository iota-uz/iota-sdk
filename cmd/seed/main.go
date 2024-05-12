package main

import (
	"database/sql"
	"github.com/iota-agency/iota-erp/models"
	"github.com/iota-agency/iota-erp/pkg/utils"
	"github.com/jmoiron/sqlx"
	"log"
)

func createInitialUser(db *sqlx.DB, email, password string) error {
	var count int
	if err := db.Get(&count, "SELECT COUNT(*) FROM users"); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	role := &models.Role{
		Name: "admin",
		Description: models.JsonNullString{
			NullString: sql.NullString{
				String: "Administrator",
				Valid:  true,
			},
		},
	}

	if err := models.Insert(db, role); err != nil {
		return err
	}
	user := &models.User{
		FirstName: "Admin",
		LastName:  "User",
		Email:     email,
		Password:  password,
	}
	if err := user.SetPassword(password); err != nil {
		return err
	}
	return models.Insert(db, user)
}

func main() {
	utils.LoadEnv()
	log.Println("Connecting to database:", utils.DbOpts())
	db, err := sqlx.Connect("postgres", utils.DbOpts())
	if err != nil {
		panic(err)
	}
	userEmail := utils.GetEnv("INITIAL_USER_EMAIL", "")
	userPassword := utils.GetEnv("INITIAL_USER_PASSWORD", "")
	if userEmail != "" && userPassword != "" {
		if err := createInitialUser(db, userEmail, userPassword); err != nil {
			panic(err)
		}
	}
}
