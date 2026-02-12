package handlers

import (
	"context"
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/iota-uz/applets"
	appletenginerpc "github.com/iota-uz/iota-sdk/pkg/appletengine/rpc"
)

type SecretsStore interface {
	Get(ctx context.Context, appletName, name string) (string, bool, error)
}

type envSecretsStore struct{}

type SecretsStub struct {
	store SecretsStore
}

func NewSecretsStub() *SecretsStub {
	return &SecretsStub{store: &envSecretsStore{}}
}

func NewSecretsStubWithStore(store SecretsStore) *SecretsStub {
	if store == nil {
		store = &envSecretsStore{}
	}
	return &SecretsStub{store: store}
}

func (s *SecretsStub) Register(registry *appletenginerpc.Registry, appletName string) error {
	router := applets.NewTypedRPCRouter()
	if err := applets.AddProcedure(router, fmt.Sprintf("%s.secrets.get", appletName), applets.Procedure[any, any]{
		Handler: func(ctx context.Context, params any) (any, error) {
			payload, ok := params.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid params: %w", applets.ErrInvalid)
			}
			name, _ := payload["name"].(string)
			name = strings.TrimSpace(name)
			if name == "" {
				return nil, fmt.Errorf("name is required: %w", applets.ErrInvalid)
			}
			value, found, err := s.store.Get(ctx, appletName, name)
			if err != nil {
				return nil, err
			}
			if !found {
				return nil, fmt.Errorf("secret not found: %w", applets.ErrNotFound)
			}
			return map[string]any{"value": value}, nil
		},
	}); err != nil {
		return err
	}
	for methodName, method := range router.Config().Methods {
		if err := registry.RegisterServerOnly(appletName, methodName, method, nil); err != nil {
			return err
		}
	}
	return nil
}

func (s *envSecretsStore) Get(_ context.Context, appletName, name string) (string, bool, error) {
	key := fmt.Sprintf("IOTA_APPLET_SECRET_%s_%s", normalizeSecretSegment(appletName), normalizeSecretSegment(name))
	value, found := os.LookupEnv(key)
	return value, found, nil
}

func normalizeSecretSegment(input string) string {
	var b strings.Builder
	b.Grow(len(input))
	for _, r := range input {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToUpper(r))
		default:
			b.WriteByte('_')
		}
	}
	return strings.Trim(b.String(), "_")
}
