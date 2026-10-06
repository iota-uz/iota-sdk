package controllers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/layouts"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

type settingsPageContext struct{ types.PageContext }

func (settingsPageContext) T(key string, _ ...map[string]interface{}) string     { return key }
func (settingsPageContext) TSafe(key string, _ ...map[string]interface{}) string { return key }
func (settingsPageContext) GetLocale() language.Tag                              { return language.English }

type settingsRuleRepo struct {
	saved bool
	rule  notifications.Rule
}

func (r *settingsRuleRepo) Get(_ context.Context, key string) (notifications.Rule, error) {
	rule := r.rule
	rule.EventKey = key
	return rule, nil
}
func (r *settingsRuleRepo) Save(context.Context, notifications.Rule) error {
	r.saved = true
	return nil
}

type settingsUsersRepo struct {
	user.Repository
	users []user.User
}

func (r *settingsUsersRepo) GetAll(context.Context) ([]user.User, error) { return r.users, nil }
func (r *settingsUsersRepo) GetByIDs(context.Context, []uint) ([]user.User, error) {
	return r.users, nil
}

func TestNotificationSettings_SaveRejectsCrossTenantRecipient(t *testing.T) {
	tenant := uuid.New()
	rules := &settingsRuleRepo{}
	catalog := notifications.NewCatalog()
	require.NoError(t, catalog.Register(notifications.TestDefinition()))
	users := &settingsUsersRepo{users: []user.User{user.New("Other", "Tenant", nil, "en", user.WithID(2), user.WithTenantID(uuid.New()))}}
	service := services.NewNotificationRoutingService(catalog, rules, users, nil, settingsAudienceRepo{})
	c := controllers.NewNotificationSettingsController().(*controllers.NotificationSettingsController)
	req := httptest.NewRequest(http.MethodPost, "/settings/notifications", strings.NewReader("event_key="+notifications.TestEventKey+"&enabled=true&user_ids=2"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Hx-Request", "true")
	ctx := composables.WithTenantID(req.Context(), tenant)
	ctx = composables.WithUser(ctx, user.New("Admin", "", nil, "en", user.WithTenantID(tenant), user.WithPermissions([]permission.Permission{permissions.NotificationRulesManage})))
	ctx = composables.WithPageCtx(ctx, settingsPageContext{})
	ctx = context.WithValue(ctx, constants.HeadKey, layouts.DefaultHead())
	bundle := i18n.NewBundle(language.English)
	for _, id := range []string{"ErrorPages.Forbidden.Message", "ErrorPages.Forbidden._Description", "ErrorPages.Forbidden.Home", "ErrorPages.Forbidden.GoBack"} {
		bundle.MustAddMessages(language.English, &i18n.Message{ID: id, Other: id})
	}
	ctx = intl.WithLocalizer(ctx, i18n.NewLocalizer(bundle, "en"))
	response := httptest.NewRecorder()
	c.Save(response, req.WithContext(ctx), service)
	require.False(t, rules.saved)
	require.Contains(t, response.Body.String(), "NotificationSettings.InvalidRule")
	require.NotContains(t, response.Body.String(), "Other Tenant")
}

func TestNotificationSettings_TestUsesSavedRule(t *testing.T) {
	tenant := uuid.New()
	catalog := notifications.NewCatalog()
	require.NoError(t, catalog.Register(notifications.TestDefinition()))
	service := services.NewNotificationRoutingService(catalog, &settingsRuleRepo{}, &settingsUsersRepo{}, nil, settingsAudienceRepo{})
	c := controllers.NewNotificationSettingsController().(*controllers.NotificationSettingsController)
	req := httptest.NewRequest(http.MethodPost, "/settings/notifications/test", strings.NewReader("enabled=true&user_ids=999"))
	req.Header.Set("Hx-Request", "true")
	ctx := composables.WithTenantID(req.Context(), tenant)
	ctx = composables.WithUser(ctx, user.New("Admin", "", nil, "en", user.WithPermissions([]permission.Permission{permissions.NotificationRulesManage})))
	ctx = composables.WithPageCtx(ctx, settingsPageContext{})
	ctx = context.WithValue(ctx, constants.HeadKey, layouts.DefaultHead())
	bundle := i18n.NewBundle(language.English)
	for _, id := range []string{"ErrorPages.Forbidden.Message", "ErrorPages.Forbidden._Description", "ErrorPages.Forbidden.Home", "ErrorPages.Forbidden.GoBack"} {
		bundle.MustAddMessages(language.English, &i18n.Message{ID: id, Other: id})
	}
	ctx = intl.WithLocalizer(ctx, i18n.NewLocalizer(bundle, "en"))
	response := httptest.NewRecorder()
	c.Test(response, req.WithContext(ctx), service)
	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), "NotificationSettings.TestSkipped")
}

func TestNotificationSettings_ReadOnlyCannotSaveOrTest(t *testing.T) {
	c := controllers.NewNotificationSettingsController().(*controllers.NotificationSettingsController)
	for _, test := range []bool{false, true} {
		req := httptest.NewRequest(http.MethodPost, "/settings/notifications", nil)
		ctx := composables.WithUser(req.Context(), user.New("Reader", "", nil, "en", user.WithPermissions([]permission.Permission{permissions.NotificationRulesRead})))
		ctx = composables.WithPageCtx(ctx, settingsPageContext{})
		ctx = context.WithValue(ctx, constants.HeadKey, layouts.DefaultHead())
		bundle := i18n.NewBundle(language.English)
		for _, id := range []string{"ErrorPages.Forbidden.Message", "ErrorPages.Forbidden._Description", "ErrorPages.Forbidden.Home", "ErrorPages.Forbidden.GoBack"} {
			bundle.MustAddMessages(language.English, &i18n.Message{ID: id, Other: id})
		}
		ctx = intl.WithLocalizer(ctx, i18n.NewLocalizer(bundle, "en"))
		response := httptest.NewRecorder()
		if test {
			c.Test(response, req.WithContext(ctx), nil)
		} else {
			c.Save(response, req.WithContext(ctx), nil)
		}
		require.Equal(t, http.StatusForbidden, response.Code)
	}
}

func TestNotificationSettings_ShowsUnavailableRecipients(t *testing.T) {
	tenant := uuid.New()
	catalog := notifications.NewCatalog()
	require.NoError(t, catalog.Register(notifications.TestDefinition()))
	service := services.NewNotificationRoutingService(catalog, &settingsRuleRepo{rule: notifications.Rule{Enabled: true, UserIDs: []uint{999}}}, &settingsUsersRepo{}, nil, settingsAudienceRepo{})
	c := controllers.NewNotificationSettingsController().(*controllers.NotificationSettingsController)
	req := httptest.NewRequest(http.MethodGet, "/settings/notifications", nil)
	req.Header.Set("Hx-Request", "true")
	ctx := composables.WithTenantID(req.Context(), tenant)
	ctx = composables.WithUser(ctx, user.New("Reader", "", nil, "en", user.WithTenantID(tenant), user.WithPermissions([]permission.Permission{permissions.NotificationRulesRead})))
	ctx = composables.WithPageCtx(ctx, settingsPageContext{})
	response := httptest.NewRecorder()
	c.Index(response, req.WithContext(ctx), service)
	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), "NotificationSettings.UnavailableRecipients")
	require.NotContains(t, response.Body.String(), "999")
}

type settingsAudienceRepo struct{}

func (settingsAudienceRepo) Groups(context.Context) ([]notifications.GroupOption, error) {
	return nil, nil
}
func (settingsAudienceRepo) Roles(context.Context) ([]notifications.RoleOption, error) {
	return nil, nil
}
func (settingsAudienceRepo) Resolve(context.Context, []uuid.UUID, []uint) ([]uint, error) {
	return nil, nil
}
