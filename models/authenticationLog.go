package models

import "time"

type AuthenticationLog struct {
	Id        int64      `gql:"id" db:"id"`
	UserId    int64      `gql:"user_id" db:"user_id"`
	Ip        string     `gql:"ip" db:"ip"`
	UserAgent string     `gql:"user_agent" db:"user_agent"`
	CreatedAt *time.Time `gql:"created_at" db:"created_at"`
}
