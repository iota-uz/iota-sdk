package auth

import "time"

type AuthenticationLog struct {
	Id        int64
	UserId    int64
	Ip        string
	UserAgent string
	CreatedAt *time.Time
}
