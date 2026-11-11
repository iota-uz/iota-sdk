package controllers_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/permissions"
	"github.com/iota-uz/iota-sdk/modules/crm/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/listext"
	"github.com/iota-uz/iota-sdk/pkg/listext/listexttest"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../../../"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

type harness struct {
	suite *itf.Suite
	fx    *listexttest.Fixture
	ids   map[string]string
}

// newHarness mounts the real CRM clients controller with the extension engine
// on the request context, as the runtime component installs it.
func newHarness(t *testing.T, activate bool, perms ...permission.Permission) *harness {
	t.Helper()
	suite := itf.NewSuite(t, modules.Components()...)
	env := suite.Env()
	fx := listexttest.Install(t, env.Ctx, env.Tx, client.ListID, listexttest.Manifest(nil), activate)

	app := env.App
	clientService := itf.GetService[services.ClientService](env)
	chatService := itf.GetService[services.ChatService](env)
	require.NotNil(t, clientService)
	c := controllers.NewClientController(app, clientService, chatService, nil, controllers.ClientControllerConfig{BasePath: "/crm/clients"})
	suite.AsUser(itf.User(perms...)).WithMiddleware(func(ctx context.Context, r *http.Request) context.Context {
		return listext.WithEngine(ctx, fx.Engine)
	}).Register(c)

	h := &harness{suite: suite, fx: fx, ids: map[string]string{}}
	sub := spi.Subject{TenantID: env.Tenant.ID, UserID: 1, Kind: spi.SubjectInitiator, Permissions: []string{"Client.Read", listexttest.VipPermission}}
	for i, name := range []string{"Ann", "Bob", "Cid"} {
		var id int64
		require.NoError(t, env.Tx.QueryRow(env.Ctx,
			`INSERT INTO clients (tenant_id, first_name, last_name, phone_number) VALUES ($1, $2, 'Doe', $3) RETURNING id`,
			env.Tenant.ID, name, fmt.Sprintf("+99890000000%d", i)).Scan(&id))
		h.ids[name] = fmt.Sprint(id)
	}
	if activate {
		fx.Put(t, env.Ctx, env.Tx, sub, h.ids["Ann"], map[string]any{"loyaltyTier": "gold", "score": 30, "vip": "platinum"})
		fx.Put(t, env.Ctx, env.Tx, sub, h.ids["Cid"], map[string]any{"loyaltyTier": "silver", "score": 10})
	}
	return h
}

func (h *harness) get(t *testing.T, q url.Values) string {
	t.Helper()
	return h.suite.GET("/crm/clients?" + q.Encode()).HTMX().HTMXTarget("clients-table-body").Expect(t).Status(200).Body()
}

func rowOrder(body string, names ...string) []string {
	type pos struct {
		name string
		at   int
	}
	var found []pos
	for _, n := range names {
		if i := strings.Index(body, n+" Doe"); i >= 0 {
			found = append(found, pos{n, i})
		}
	}
	for i := range found {
		for j := i + 1; j < len(found); j++ {
			if found[j].at < found[i].at {
				found[i], found[j] = found[j], found[i]
			}
		}
	}
	out := make([]string, len(found))
	for i, f := range found {
		out[i] = f.name
	}
	return out
}

func vipPermission() permission.Permission {
	return permission.New(permission.WithName(listexttest.VipPermission))
}

func TestClientsListShowsOverlayColumnWithFilterAndSort(t *testing.T) {
	h := newHarness(t, true, permissions.ClientRead, vipPermission())
	names := []string{"Ann", "Bob", "Cid"}

	body := h.suite.GET("/crm/clients").Expect(t).Status(200).Body()
	assert.Contains(t, body, "Loyalty tier", "header of the contributed column")
	assert.Contains(t, body, `name="ext.`+listexttest.TierCol+`"`, "its filter control")
	assert.Contains(t, body, `name="ext.sort"`, "sort control")
	assert.Contains(t, body, "platinum", "VIP is readable by this user")
	assert.Contains(t, body, `data-ext-cell="`+listexttest.TierCol+`"`)
	assert.Contains(t, body, ">gold<")
	assert.Contains(t, body, ">none<", "a client without an overlay row reads the declared default")

	q := url.Values{}
	q.Set("ext."+listexttest.TierCol, "silver")
	assert.Equal(t, []string{"Cid"}, rowOrder(h.get(t, q), names...))

	q = url.Values{}
	q.Set("ext.sort", listexttest.ScoreCol)
	q.Set("ext.dir", "desc")
	assert.Equal(t, []string{"Ann", "Cid", "Bob"}, rowOrder(h.get(t, q), names...), "NULL scores sort last when descending")

	q = url.Values{}
	q.Set("extfrom."+listexttest.ScoreCol, "11")
	assert.Equal(t, []string{"Ann"}, rowOrder(h.get(t, q), names...))
}

func TestClientsListHidesFieldsTheUserMayNotRead(t *testing.T) {
	h := newHarness(t, true, permissions.ClientRead)
	body := h.suite.GET("/crm/clients").Expect(t).Status(200).Body()
	assert.Contains(t, body, "Loyalty tier")
	assert.NotContains(t, body, `name="ext.`+listexttest.VipCol+`"`)
	assert.NotContains(t, body, "platinum", "the vip field needs Extra.Vip")
	// ... and cannot be used as a filter or as an ordering
	q := url.Values{}
	q.Set("ext."+listexttest.VipCol, "platinum")
	assert.Equal(t, []string{"Ann", "Bob", "Cid"}, rowOrder(h.get(t, q), "Ann", "Bob", "Cid"))
}

func TestClientsListWithoutPackageIsUnchanged(t *testing.T) {
	h := newHarness(t, false, permissions.ClientRead)
	body := h.suite.GET("/crm/clients").Expect(t).Status(200).Body()
	assert.NotContains(t, body, "Loyalty tier")
	assert.NotContains(t, body, `name="ext.sort"`)
	assert.Contains(t, body, "Ann Doe")
	// a stale filter parameter changes nothing
	q := url.Values{}
	q.Set("ext."+listexttest.TierCol, "gold")
	assert.Equal(t, []string{"Ann", "Bob", "Cid"}, rowOrder(h.get(t, q), "Ann", "Bob", "Cid"))
}

// lastUpload returns the content of the most recent upload of the test tx.
func lastUpload(t *testing.T, h *harness) []byte {
	t.Helper()
	env := h.suite.Env()
	var path string
	require.NoError(t, env.Tx.QueryRow(env.Ctx, `SELECT path FROM uploads WHERE tenant_id = $1 ORDER BY id DESC LIMIT 1`, env.Tenant.ID).Scan(&path))
	b, err := os.ReadFile(path)
	require.NoError(t, err, path)
	return b
}

func TestClientsExportCarriesOverlayColumns(t *testing.T) {
	h := newHarness(t, true, permissions.ClientRead, vipPermission())

	post := func(format string, q url.Values) {
		q.Set("format", format)
		h.suite.POST("/crm/clients/export?" + q.Encode()).Expect(t).Status(303)
	}

	post("csv", url.Values{})
	csv := strings.TrimPrefix(string(lastUpload(t, h)), "\ufeff")
	lines := strings.Split(strings.TrimSpace(csv), "\n")
	require.Len(t, lines, 4, csv)
	assert.Contains(t, lines[0], "Loyalty tier")
	assert.Contains(t, lines[0], "Score")
	assert.Contains(t, lines[0], "VIP")
	assert.NotContains(t, lines[0], "Notes", "a column that is not exportable stays out of the export")
	assert.Contains(t, csv, "gold")
	assert.Contains(t, csv, "platinum")
	assert.Contains(t, csv, "none", "the default-read value is exported for a client without an overlay row")

	// the same list filters apply to the exported rows
	post("csv", url.Values{"ext." + listexttest.TierCol: {"gold"}})
	csv = string(lastUpload(t, h))
	assert.Contains(t, csv, "Ann")
	assert.NotContains(t, csv, "Cid")
	assert.NotContains(t, csv, "Bob")

	post("excel", url.Values{})
	xlsx := lastUpload(t, h)
	rows := readSheet(t, xlsx)
	require.Len(t, rows, 4)
	assert.Contains(t, rows[0], "Loyalty tier")
	var annRow []string
	for _, r := range rows {
		if len(r) > 0 && strings.HasPrefix(r[0], "Ann") {
			annRow = r
		}
	}
	require.NotNil(t, annRow)
	assert.Contains(t, annRow, "gold")
	assert.Contains(t, annRow, "30")
}

func TestClientsExportWithoutPackageKeepsStandardColumns(t *testing.T) {
	h := newHarness(t, false, permissions.ClientRead)
	h.suite.POST("/crm/clients/export?format=csv").Expect(t).Status(303)
	csv := string(lastUpload(t, h))
	lines := strings.Split(strings.TrimSpace(csv), "\n")
	assert.Len(t, strings.Split(lines[0], ","), 3)
}

func TestClientsListDeniedWithoutClientRead(t *testing.T) {
	h := newHarness(t, true)
	h.suite.GET("/crm/clients").Expect(t).Status(403)
}

func readSheet(t *testing.T, content []byte) [][]string {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(content))
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	rows, err := f.GetRows(f.GetSheetName(0))
	require.NoError(t, err)
	return rows
}
