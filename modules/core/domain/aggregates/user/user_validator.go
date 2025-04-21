package user

import (
	"context"
)

type Validator interface {
	ValidateCreate(ctx context.Context, u User) error
	ValidateUpdate(ctx context.Context, u User) error
}
