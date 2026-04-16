// Package bootstrap provides runtime initialization and composition helpers for SDK entrypoints.
package bootstrap

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/iota-uz/applets"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/text/language"
)

type sdkAppletUserAdapter struct {
	user user.User
}

func (a *sdkAppletUserAdapter) ID() uint {
	return a.user.ID()
}

func (a *sdkAppletUserAdapter) DisplayName() string {
	return strings.TrimSpace(a.user.FirstName() + " " + a.user.LastName())
}

func (a *sdkAppletUserAdapter) HasPermission(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return false
	}
	for _, permissionName := range composables.EffectivePermissionNames(a.user) {
		if strings.ToLower(permissionName) == name {
			return true
		}
	}
	return false
}

func (a *sdkAppletUserAdapter) PermissionNames() []string {
	return composables.EffectivePermissionNames(a.user)
}

type sdkHostServices struct {
	pool *pgxpool.Pool
}

func NewSDKHostServices(pool *pgxpool.Pool) applets.HostServices {
	return &sdkHostServices{pool: pool}
}

func (h *sdkHostServices) ExtractUser(ctx context.Context) (applets.AppletUser, error) {
	currentUser, err := composables.UseUser(ctx)
	if err != nil || currentUser == nil {
		return nil, err
	}
	return &sdkAppletUserAdapter{user: currentUser}, nil
}

func (h *sdkHostServices) ExtractTenantID(ctx context.Context) (uuid.UUID, error) {
	return composables.UseTenantID(ctx)
}

func (h *sdkHostServices) ExtractPool(context.Context) (*pgxpool.Pool, error) {
	if h.pool == nil {
		return nil, fmt.Errorf("pool is not configured")
	}
	return h.pool, nil
}

func (h *sdkHostServices) ExtractPageLocale(ctx context.Context) language.Tag {
	pageContext, ok := ctx.Value(constants.PageContext).(types.PageContext)
	if !ok {
		return language.English
	}
	return pageContext.GetLocale()
}
