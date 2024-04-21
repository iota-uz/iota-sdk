package models

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	"strings"
	"time"
)

type Role struct {
	Id          int64          `gql:"id" db:"id"`
	Name        string         `gql:"name" db:"name"`
	Description JsonNullString `gql:"description" db:"description"`
	CreatedAt   *time.Time     `gql:"created_at" db:"created_at"`
	UpdatedAt   *time.Time     `gql:"updated_at" db:"updated_at"`
}

func (r *Role) fields() []string {
	return []string{"name", "description"}
}

func (r *Role) insert(db *sqlx.DB) error {
	q := fmt.Sprintf(
		"INSERT INTO roles (%s) VALUES (:%s) RETURNING id, created_at, updated_at",
		strings.Join(r.fields(), ", "),
		strings.Join(r.fields(), ", :"),
	)
	stmt, err := db.PrepareNamed(q)
	if err != nil {
		return err
	}
	defer stmt.Close()
	return stmt.QueryRow(r).Scan(&r.Id, &r.CreatedAt, &r.UpdatedAt)
}

func (r *Role) update(db *sqlx.DB) error {
	t := time.Now()
	r.UpdatedAt = &t
	var fields []string
	for _, field := range r.fields() {
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
	_, err = stmt.Exec(r)
	return err
}

func (r *Role) Save(db *sqlx.DB) error {
	if r.Id == 0 {
		return r.insert(db)
	}
	return r.update(db)
}
