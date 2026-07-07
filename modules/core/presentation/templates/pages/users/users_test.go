package users

import (
	"context"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/components/base"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

// stubPageCtx satisfies types.PageContext returning the key itself for every
// translation call, so cell/row templates render outside a full HTTP request.
type stubPageCtx struct{}

func (stubPageCtx) T(key string, _ ...map[string]interface{}) string     { return key }
func (stubPageCtx) TSafe(key string, _ ...map[string]interface{}) string { return key }
func (s stubPageCtx) Namespace(string) types.PageContext                 { return s }
func (stubPageCtx) ToJSLocale() string                                   { return "en-US" }
func (stubPageCtx) GetLocale() language.Tag                              { return language.English }
func (stubPageCtx) GetURL() *url.URL                                     { return &url.URL{Path: "/users"} }
func (stubPageCtx) GetLocalizer() *i18n.Localizer                        { return nil }

func usersTestCtx() context.Context {
	return composables.WithPageCtx(context.Background(), stubPageCtx{})
}

func testUser() *viewmodels.User {
	return &viewmodels.User{
		ID:        "user-42",
		FirstName: "Jane",
		LastName:  "Doe",
		Email:     "jane@example.com",
		CreatedAt: "2024-03-15T00:00:00Z",
		Roles:     []*viewmodels.Role{{ID: "1", Name: "Admin"}},
		CanUpdate: true,
	}
}

// parseTableFragment wraps a rendered <tr>/<tbody> fragment in a <table> tag
// before parsing: the HTML5 tree-construction algorithm foster-parents (drops)
// bare <tr>/<tbody> elements found outside table context, which would
// otherwise make goquery.Find("tr")/("tbody") report zero matches even though
// the fragment is well-formed on its own.
func parseTableFragment(t *testing.T, fragment string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<table>" + fragment + "</table>"))
	require.NoError(t, err)
	return doc
}

// TestUserTableRow_RowID verifies userTableRow (used by BuildTableConfig's
// full list render) sets the "user-<id>" row id the realtime OOB path relies
// on to match rows for update/delete swaps.
func TestUserTableRow_RowID(t *testing.T) {
	t.Parallel()

	row := userTableRow(testUser())
	assert.Equal(t, "user-user-42", row.Attrs()["id"])
	require.Len(t, row.Cells(), 4)
}

// TestUserRow_MatchesListRowID verifies the realtime single-row OOB render
// (UserRow) produces the same row id as the full list render (userTableRow),
// which is required for htmx.ws.js's outerHTML-by-id update/delete matching.
func TestUserRow_MatchesListRowID(t *testing.T) {
	t.Parallel()

	user := testUser()
	listRow := userTableRow(user)

	var b strings.Builder
	err := UserRow(user, &base.TableRowProps{Attrs: map[string]any{}}).Render(usersTestCtx(), &b)
	require.NoError(t, err)

	doc := parseTableFragment(t, b.String())

	tr := doc.Find("tr")
	require.Equal(t, 1, tr.Length())
	id, ok := tr.Attr("id")
	require.True(t, ok)
	assert.Equal(t, listRow.Attrs()["id"], id)
}

// TestUserRow_RendersUserContent verifies the OOB row actually renders the
// user's data (title, role, actions) rather than an empty/placeholder row.
func TestUserRow_RendersUserContent(t *testing.T) {
	t.Parallel()

	user := testUser()

	var b strings.Builder
	err := UserRow(user, &base.TableRowProps{Attrs: map[string]any{}}).Render(usersTestCtx(), &b)
	require.NoError(t, err)

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(b.String()))
	require.NoError(t, err)

	html := doc.Text()
	assert.Contains(t, html, "Admin")

	viewLink := doc.Find(`a[href="/users/user-42"]`)
	assert.Equal(t, 1, viewLink.Length(), "expected view action link")

	editLink := doc.Find(`a[href="/users/user-42/edit"]`)
	assert.Equal(t, 1, editLink.Length(), "expected edit action link")
}

// TestUserCreatedEvent_OOBTarget verifies the realtime "row created" broadcast
// targets #table-body (sfui's hardcoded tbody id), not the old
// #users-table-body id the legacy base.Table implementation used.
func TestUserCreatedEvent_OOBTarget(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	err := UserCreatedEvent(testUser(), &base.TableRowProps{Attrs: map[string]any{}}).Render(usersTestCtx(), &b)
	require.NoError(t, err)

	doc := parseTableFragment(t, b.String())

	tbody := doc.Find("tbody")
	require.Equal(t, 1, tbody.Length())
	swapOOB, ok := tbody.Attr("hx-swap-oob")
	require.True(t, ok)
	assert.Equal(t, "afterbegin:#table-body", swapOOB)
}

// TestBuildTableConfig_ActionGating verifies the "New user" action (folded
// into a single cfg.AddActions call site after the sidebar removal) is
// omitted entirely for a context with a user lacking UserCreate permission,
// and present when permission is granted. composables.CanUser treats an
// absent user as permitted (fail-open for internal/system callers), so this
// deliberately exercises both branches with an explicit user in context
// rather than relying on an empty context to mean "no permission".
func TestBuildTableConfig_ActionGating(t *testing.T) {
	t.Parallel()

	props := &IndexPageProps{
		Users:   []*viewmodels.User{testUser()},
		Page:    1,
		PerPage: 25,
	}
	req := httptest.NewRequest("GET", "/users", nil)

	email, err := internet.NewEmail("action-gating@example.com")
	require.NoError(t, err)

	noPermUser := user.New("No", "Perm", email, user.UILanguageEN)
	cfgNoPerm := BuildTableConfig(composables.WithUser(usersTestCtx(), noPermUser), props, req)
	require.Len(t, cfgNoPerm.Rows, 1)
	// Only the help link action is present without UserCreate.
	assert.Len(t, cfgNoPerm.Actions, 1)

	permUser := user.New("Has", "Perm", email, user.UILanguageEN, user.WithPermissions([]permission.Permission{permissions.UserCreate}))
	cfgWithPerm := BuildTableConfig(composables.WithUser(usersTestCtx(), permUser), props, req)
	// Help link + "New user" button are both present with UserCreate.
	assert.Len(t, cfgWithPerm.Actions, 2)
}
