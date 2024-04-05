package models

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
	"strings"
	"time"
)

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"error"`
}

func (v *ValidationError) Error() string {
	return v.Message
}

func NewValidationError(field, err string) *ValidationError {
	return &ValidationError{Field: field, Message: err}
}

type User struct {
	Id         int64          `json:"id" db:"id"`
	FirstName  string         `json:"first_name" db:"first_name"`
	LastName   string         `json:"last_name" db:"last_name"`
	MiddleName JsonNullString `json:"middle_name,omitempty" db:"middle_name"`
	Password   string         `json:"-" db:"password"`
	Email      string         `json:"email" db:"email"`
	RoleId     int64          `json:"role_id" db:"role_id"`
	LastIp     JsonNullString `json:"last_ip" db:"last_ip"`
	LastLogin  *time.Time     `json:"last_login" db:"last_login"`
	LastAction *time.Time     `json:"last_action" db:"last_action"`
	CreatedAt  *time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt  *time.Time     `json:"updated_at" db:"updated_at"`
}

func (u *User) fields() []string {
	return []string{"first_name", "last_name", "middle_name", "password", "email", "last_ip", "last_login", "last_action"}
}

func (u *User) CheckPassword(password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)) == nil
}

func (u *User) Validate() []*ValidationError {
	var errs []*ValidationError
	if u.FirstName == "" {
		errs = append(errs, NewValidationError("first_name", "first_name is required"))
	}
	if u.LastName == "" {
		errs = append(errs, NewValidationError("last_name", "last_name is required"))
	}
	if u.Email == "" {
		errs = append(errs, NewValidationError("email", "email is required"))
	}
	if u.Password == "" {
		errs = append(errs, NewValidationError("password", "password is required"))
	}
	return errs
}

func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hash)
	return nil
}

func (u *User) insert(db *sqlx.DB) error {
	q := fmt.Sprintf(
		"INSERT INTO users (%s) VALUES (:%s) RETURNING id, created_at, updated_at",
		strings.Join(u.fields(), ", "),
		strings.Join(u.fields(), ", :"),
	)
	stmt, err := db.PrepareNamed(q)
	if err != nil {
		return err
	}
	defer stmt.Close()
	return stmt.QueryRow(u).Scan(&u.Id, &u.CreatedAt, &u.UpdatedAt)
}

func (u *User) update(db *sqlx.DB) error {
	t := time.Now()
	u.UpdatedAt = &t
	var fields []string
	for _, field := range u.fields() {
		fields = append(fields, fmt.Sprintf("%s = :%s", field, field))
	}
	q := fmt.Sprintf(
		"UPDATE users SET %s WHERE id = :id",
		strings.Join(fields, ", "),
	)
	stmt, err := db.PrepareNamed(q)
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(u)
	return err
}

func (u *User) Save(db *sqlx.DB) error {
	if u.Id == 0 {
		return u.insert(db)
	}
	return u.update(db)
}
