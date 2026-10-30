package controllers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
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

func (r *settingsUsersRepo) GetByIDs(context.Context, []uint) ([]user.User, error) {
	return r.users, nil
}

func TestNotificationSettings_SaveRejectsCrossTenantRecipient(t *testing.T) {
	tenant := uuid.New()
	rules := &settingsRuleRepo{}
	catalog := notifications.NewCatalog()
	require.NoError(t, catalog.Register(notifications.TestDefinition()))
	users := &settingsUsersRepo{users: []user.User{user.New("Other", "Tenant", nil, "en", user.WithID(2), user.WithTenantID(uuid.New()))}}
	service := services.NewNotificationRoutingService(catalog, rules, users, nil, settingsAudienceRepo{}, &settingsDirectory{allowed: []notifications.Recipient{{ID: 2, Name: "Other Tenant"}}})
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
	service := services.NewNotificationRoutingService(catalog, &settingsRuleRepo{}, &settingsUsersRepo{}, nil, settingsAudienceRepo{}, &settingsDirectory{})
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
	c.Test(response, req.WithContext(ctx), service, services.NewNotificationDispatchService(service, nil, nil))
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
			c.Test(response, req.WithContext(ctx), nil, nil)
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
	service := services.NewNotificationRoutingService(catalog, &settingsRuleRepo{rule: notifications.Rule{Enabled: true, UserIDs: []uint{999}}}, &settingsUsersRepo{}, nil, settingsAudienceRepo{}, &settingsDirectory{})
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
	require.Contains(t, response.Body.String(), `&#34;id&#34;:&#34;999&#34;`)
	require.Contains(t, response.Body.String(), `&#34;gone&#34;:true`)
}

type settingsDirectory struct {
	allowed  []notifications.Recipient
	searched []notifications.RecipientQuery
}

func (d *settingsDirectory) Search(_ context.Context, query notifications.RecipientQuery) ([]notifications.Recipient, error) {
	d.searched = append(d.searched, query)
	if query.Offset >= len(d.allowed) {
		return []notifications.Recipient{}, nil
	}
	return d.allowed[query.Offset:min(query.Offset+query.Limit, len(d.allowed))], nil
}
func (d *settingsDirectory) GetByIDs(_ context.Context, ids []uint) ([]notifications.Recipient, error) {
	result := []notifications.Recipient{}
	for _, recipient := range d.allowed {
		if slices.Contains(ids, recipient.ID) {
			result = append(result, recipient)
		}
	}
	return result, nil
}

func settingsRequest(t *testing.T, method, target string, perms ...permission.Permission) *http.Request {
	t.Helper()
	tenant := uuid.New()
	req := httptest.NewRequest(method, target, nil)
	req.Header.Set("Hx-Request", "true")
	ctx := composables.WithTenantID(req.Context(), tenant)
	ctx = composables.WithUser(ctx, user.New("Admin", "", nil, "en", user.WithTenantID(tenant), user.WithPermissions(perms)))
	ctx = composables.WithPageCtx(ctx, settingsPageContext{})
	ctx = context.WithValue(ctx, constants.HeadKey, layouts.DefaultHead())
	bundle := i18n.NewBundle(language.English)
	for _, id := range []string{"ErrorPages.Forbidden.Message", "ErrorPages.Forbidden._Description", "ErrorPages.Forbidden.Home", "ErrorPages.Forbidden.GoBack"} {
		bundle.MustAddMessages(language.English, &i18n.Message{ID: id, Other: id})
	}
	ctx = intl.WithLocalizer(ctx, i18n.NewLocalizer(bundle, "en"))
	return req.WithContext(ctx)
}

// The page once embedded the whole directory in every rule form, which on a
// 39k-user tenant hung the browser. Falsely green if the directory fake had no
// bystander: Unlisted Person exists and must reach the page only through search.
func TestNotificationSettings_PageEmbedsOnlyNamedRecipients(t *testing.T) {
	catalog := notifications.NewCatalog()
	require.NoError(t, catalog.Register(notifications.TestDefinition()))
	directory := &settingsDirectory{allowed: []notifications.Recipient{{ID: 7, Name: "Named Person", Hint: "named@example.test"}, {ID: 8, Name: "Unlisted Person", Hint: "unlisted@example.test"}}}
	service := services.NewNotificationRoutingService(catalog, &settingsRuleRepo{rule: notifications.Rule{Enabled: true, UserIDs: []uint{7}}}, &settingsUsersRepo{}, nil, settingsAudienceRepo{}, directory)
	c := controllers.NewNotificationSettingsController().(*controllers.NotificationSettingsController)
	response := httptest.NewRecorder()
	c.Index(response, settingsRequest(t, http.MethodGet, "/settings/notifications", permissions.NotificationRulesRead, permissions.NotificationRulesManage), service)
	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), "Named Person")
	require.NotContains(t, response.Body.String(), "Unlisted Person")
	require.Empty(t, directory.searched)
}

// Falsely green if the page size were not enforced by the service: the fake
// holds one person more than a page and honours whatever limit it is asked for.
func TestNotificationSettings_RecipientsArePaged(t *testing.T) {
	recipients := make([]notifications.Recipient, 21)
	for i := range recipients {
		recipients[i] = notifications.Recipient{ID: uint(i + 1), Name: fmt.Sprintf("Person %02d", i+1)}
	}
	directory := &settingsDirectory{allowed: recipients}
	service := services.NewNotificationRoutingService(notifications.NewCatalog(), &settingsRuleRepo{}, &settingsUsersRepo{}, nil, settingsAudienceRepo{}, directory)
	c := controllers.NewNotificationSettingsController().(*controllers.NotificationSettingsController)
	type page struct {
		Items []struct {
			Kind string `json:"kind"`
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
		HasMore bool `json:"has_more"`
	}
	fetch := func(target string) page {
		response := httptest.NewRecorder()
		c.Recipients(response, settingsRequest(t, http.MethodGet, target, permissions.NotificationRulesManage), service)
		require.Equal(t, http.StatusOK, response.Code)
		var got page
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &got))
		return got
	}
	first := fetch("/settings/notifications/recipients?q=ann")
	require.Len(t, first.Items, 20)
	require.True(t, first.HasMore)
	require.Equal(t, "user", first.Items[0].Kind)
	require.Equal(t, "ann", directory.searched[0].Search)
	second := fetch("/settings/notifications/recipients?offset=20")
	require.Len(t, second.Items, 1)
	require.False(t, second.HasMore)
	require.Equal(t, "Person 21", second.Items[0].Name)
}

// Falsely green if the check only hid the picker: the endpoint itself refuses.
func TestNotificationSettings_RecipientsRequireManage(t *testing.T) {
	directory := &settingsDirectory{allowed: []notifications.Recipient{{ID: 1, Name: "Hidden Person"}}}
	service := services.NewNotificationRoutingService(notifications.NewCatalog(), &settingsRuleRepo{}, &settingsUsersRepo{}, nil, settingsAudienceRepo{}, directory)
	c := controllers.NewNotificationSettingsController().(*controllers.NotificationSettingsController)
	response := httptest.NewRecorder()
	c.Recipients(response, settingsRequest(t, http.MethodGet, "/settings/notifications/recipients", permissions.NotificationRulesRead), service)
	require.Equal(t, http.StatusForbidden, response.Code)
	require.Empty(t, directory.searched)
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

func TestNotificationSettings_ReadOnlyCannotRetryOrEnqueue(t *testing.T) {
	c := controllers.NewNotificationSettingsController().(*controllers.NotificationSettingsController)
	for _, enqueue := range []bool{false, true} {
		req := httptest.NewRequest(http.MethodPost, "/settings/notifications/queue/retry", nil)
		ctx := composables.WithUser(req.Context(), user.New("Reader", "", nil, "en", user.WithPermissions([]permission.Permission{permissions.NotificationRulesRead})))
		ctx = composables.WithPageCtx(ctx, settingsPageContext{})
		ctx = context.WithValue(ctx, constants.HeadKey, layouts.DefaultHead())
		bundle := i18n.NewBundle(language.English)
		for _, id := range []string{"ErrorPages.Forbidden.Message", "ErrorPages.Forbidden._Description", "ErrorPages.Forbidden.Home", "ErrorPages.Forbidden.GoBack"} {
			bundle.MustAddMessages(language.English, &i18n.Message{ID: id, Other: id})
		}
		ctx = intl.WithLocalizer(ctx, i18n.NewLocalizer(bundle, "en"))
		response := httptest.NewRecorder()
		if enqueue {
			c.Test(response, req.WithContext(ctx), nil, nil)
		} else {
			c.Retry(response, req.WithContext(ctx), nil)
		}
		require.Equal(t, http.StatusForbidden, response.Code)
	}
}
