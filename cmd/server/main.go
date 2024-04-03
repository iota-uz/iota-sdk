package main

import (
	"database/sql"
	"github.com/apollos-studio/sso/models"
	"github.com/apollos-studio/sso/pkg/server"
	"github.com/apollos-studio/sso/pkg/utils"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
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
	if err := role.Save(db); err != nil {
		return err
	}
	user := &models.User{
		FirstName: "Admin",
		LastName:  "User",
		Email:     email,
		Password:  password,
		RoleId:    role.Id,
	}
	if err := user.SetPassword(password); err != nil {
		return err
	}
	return user.Save(db)
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
	srv := server.New(db)
	srv.Start()
}
