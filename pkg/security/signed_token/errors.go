package signed_token

import "errors"

var (
	ErrInvalid = errors.New("signed token: invalid signature or format")
	ErrExpired = errors.New("signed token: expired")
)
