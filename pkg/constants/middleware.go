package constants

import "github.com/go-playground/validator/v10"

type UserContextKey string

const (
	UserKey    UserContextKey = "user"
	SessionKey UserContextKey = "session"
)

var (
	Validate = validator.New(validator.WithRequiredStructEnabled())
)
