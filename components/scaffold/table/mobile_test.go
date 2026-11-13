package table

import (
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func renderIn(t *testing.T, ctx context.Context, c templ.Component) string {
	t.Helper()
	var out strings.Builder
	require.NoError(t, c.Render(ctx, &out))
	return out.String()
}

func financeConfig() *TableConfig {
	cfg := NewTableConfig("Transfers", "/transfers")
	cfg.Configurable = false
	cfg.AddCols(
		Column("date", "Date"),
		Column("category", "Category"),
		MoneyColumn("amount", "Amount"),
		Column("comment", "Comment", WithPriority(3)),
		Column("note", "Note", WithMobileRole(MobileRoleHidden)),
		Column("actions", "", WithSticky(StickyPositionRight)),
	)
	return cfg
}

func TestMobileRoleDefaults(t *testing.T) {
	cfg := financeConfig()
	got := map[string]string{}
	for _, col := range cfg.Columns {
		got[col.Key()] = cfg.mobileRoleAttr(col)
	}
	assert.Equal(t, map[string]string{
		"date":     "primary",
		"category": "secondary",
		"amount":   "amount",
		"comment":  "hidden-mobile",
		"note":     "hidden-mobile",
		"actions":  "actions",
	}, got)
}

func TestActionsKeyedColumnIsNotTheTitle(t *testing.T) {
	cfg := NewTableConfig("Transfers", "/transfers")
	cfg.AddCols(Column("actions", ""), Column("date", "Date"))
	assert.Equal(t, "actions", cfg.mobileRoleAttr(cfg.Columns[0]))
	assert.Equal(t, "primary", cfg.mobileRoleAttr(cfg.Columns[1]))
}

func TestMobileRoleDeclaredPrimaryWins(t *testing.T) {
	cfg := NewTableConfig("Employees", "/employees")
	cfg.AddCols(
		Column("id", "ID", WithMobileRole(MobileRoleHidden)),
		Column("name", "Name", WithMobileRole(MobileRolePrimary)),
		Column("phone", "Phone"),
	)
	assert.Equal(t, MobileRoleHidden, cfg.resolvedMobileRole(cfg.Columns[0]))
	assert.Equal(t, MobileRolePrimary, cfg.resolvedMobileRole(cfg.Columns[1]))
	assert.Equal(t, MobileRoleSecondary, cfg.resolvedMobileRole(cfg.Columns[2]))
}

func TestMobileRoleSkipsHiddenFirstColumnForPrimary(t *testing.T) {
	cfg := NewTableConfig("Employees", "/employees")
	cfg.AddCols(
		Column("id", "ID", WithMobileRole(MobileRoleHidden)),
		Column("name", "Name"),
	)
	assert.Equal(t, MobileRolePrimary, cfg.resolvedMobileRole(cfg.Columns[1]))
}

func TestMobileRoleInvalidIsIgnored(t *testing.T) {
	col := Column("a", "A", WithMobileRole("sideways"))
	assert.Equal(t, MobileRoleAuto, columnMobileRole(col))
}

// A column type written before roles existed has no MobileRole method and must
// keep working as an automatic column.
type legacyColumn struct{ TableColumn }

func TestMobileRoleForColumnWithoutRoleSupport(t *testing.T) {
	cfg := NewTableConfig("Legacy", "/legacy")
	cfg.AddCols(legacyColumn{Column("a", "A")}, legacyColumn{Column("b", "B")})
	assert.Equal(t, MobileRolePrimary, cfg.resolvedMobileRole(cfg.Columns[0]))
	assert.Equal(t, MobileRoleSecondary, cfg.resolvedMobileRole(cfg.Columns[1]))
}

func TestRowsCarryMobileRolesAndRowLinkMarker(t *testing.T) {
	ctx := testPageCtx(t, "/transfers")
	cfg := financeConfig()
	cell := func(v string) TableCell { return Cell(templ.Raw(v), v) }
	cfg.Rows = []TableRow{
		Row(cell("01.02"), cell("Salary"), cell("100"), cell("c"), cell("n"), cell("act")).ApplyOpts(WithDrawer("/transfers/1")),
		Row(cell("03.02"), cell("Rent"), cell("200"), cell("c"), cell("n"), cell("act")),
	}
	out := renderIn(t, ctx, Rows(cfg))

	for _, role := range []string{"primary", "secondary", "amount", "hidden-mobile", "actions"} {
		assert.Contains(t, out, `data-mobile-role="`+role+`"`, role)
	}
	assert.Equal(t, 1, strings.Count(out, `data-row-link="true"`), "only the row that opens a record is marked")
}

func TestTableUsesCardLayoutByDefault(t *testing.T) {
	ctx := testPageCtx(t, "/transfers")
	for name, tc := range map[string]struct {
		mutate func(*TableConfig)
		cards  bool
	}{
		"default":  {func(*TableConfig) {}, true},
		"opt out":  {func(c *TableConfig) { c.MobileLayout = MobileLayoutTable }, false},
		"forced":   {func(c *TableConfig) { c.MobileLayout = MobileLayoutCards }, true},
		"editable": {func(c *TableConfig) { c.Editable.Enabled = true; c.Editable.Key = "rows" }, false},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := financeConfig()
			tc.mutate(cfg)
			out := renderIn(t, ctx, Table(cfg))
			assert.Equal(t, tc.cards, strings.Contains(out, "table-cards"))
		})
	}
}

func TestFooterCellsCarryMobileRoles(t *testing.T) {
	ctx := testPageCtx(t, "/transfers")
	cfg := financeConfig()
	cfg.Footer = []TableCell{Cell(templ.Raw("Total"), nil), Cell(templ.Raw(""), nil), Cell(templ.Raw("300"), nil)}
	out := renderIn(t, ctx, tableFooter(cfg))
	assert.Contains(t, out, `data-mobile-role="primary"`)
	assert.Contains(t, out, `data-mobile-role="amount"`)
}

func TestEmbeddedContentShowsActionsOnMobile(t *testing.T) {
	ctx := testPageCtx(t, "/transfers")
	cfg := financeConfig()
	cfg.Actions = []templ.Component{
		templ.Raw(`<button id="create">Create</button>`),
		templ.Raw(`<button id="export">Export</button>`),
		templ.Raw(`<button id="import">Import</button>`),
	}

	out := renderIn(t, ctx, EmbeddedContent(cfg))

	assert.Contains(t, out, "data-mobile-actions")
	bar := out[strings.Index(out, "data-mobile-actions"):]
	bar = bar[:strings.Index(bar, "</form>")+len("</form>")]
	assert.Equal(t, 1, strings.Count(bar[:strings.Index(bar, `x-data="{ open: false }"`)], `id="create"`), "primary action is a visible button")
	menu := bar[strings.Index(bar, `role="menu"`):]
	assert.Contains(t, menu, `id="export"`)
	assert.Contains(t, menu, `id="import"`)
	assert.NotContains(t, menu[:strings.Index(menu, `id="import"`)], `id="create"`)
	assert.Contains(t, out, `aria-label="Scaffold.Table.MoreActions"`)
}

func TestSingleActionHasNoMoreMenu(t *testing.T) {
	ctx := testPageCtx(t, "/transfers")
	cfg := financeConfig()
	cfg.Actions = []templ.Component{templ.Raw(`<button id="create">Create</button>`)}
	out := renderIn(t, ctx, EmbeddedContent(cfg))
	assert.Contains(t, out, `id="create"`)
	assert.NotContains(t, out, `role="menu"`)
}

func TestContentDoesNotRepeatMobileActions(t *testing.T) {
	ctx := testPageCtx(t, "/transfers")
	cfg := financeConfig()
	cfg.Actions = []templ.Component{templ.Raw(`<button>Create</button>`), templ.Raw(`<button>Export</button>`)}
	out := renderIn(t, ctx, Content(cfg))
	assert.Equal(t, 1, strings.Count(out, "data-mobile-actions"))
}

func TestMobilePrimaryActionIndex(t *testing.T) {
	cfg := NewTableConfig("T", "/t", WithMobilePrimaryAction(1))
	cfg.Actions = []templ.Component{templ.Raw("help"), templ.Raw("create"), templ.Raw("export")}
	primary, more := cfg.splitMobileActions()
	assert.Equal(t, "create", renderIn(t, context.Background(), primary))
	require.Len(t, more, 2)

	cfg.MobilePrimaryAction = 9
	primary, more = cfg.splitMobileActions()
	assert.Equal(t, "help", renderIn(t, context.Background(), primary))
	assert.Len(t, more, 2)
}

func TestEmptyStateDistinguishesNoRecordsFromNothingFound(t *testing.T) {
	for name, tc := range map[string]struct {
		target string
		search string
		want   string
	}{
		"no records":       {"/transfers", "", "Scaffold.Table.NoDataTitle"},
		"paging only":      {"/transfers?page=1&limit=20&sort=date&order=asc", "", "Scaffold.Table.NoDataTitle"},
		"blank filter":     {"/transfers?category=", "", "Scaffold.Table.NoDataTitle"},
		"filter":           {"/transfers?category=rent", "", "Scaffold.Table.NothingFound"},
		"search":           {"/transfers", "salary", "Scaffold.Table.NothingFound"},
		"search from form": {"/transfers?Search=salary", "", "Scaffold.Table.NothingFound"},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := testPageCtx(t, tc.target)
			cfg := financeConfig()
			cfg.SearchValue = tc.search
			for _, c := range []templ.Component{Rows(cfg), Table(cfg)} {
				out := renderIn(t, ctx, c)
				assert.Contains(t, out, tc.want)
			}
		})
	}
}

func TestCellLinkSkipsPlaceholders(t *testing.T) {
	ctx := testPageCtx(t, "/transfers")
	col := Column("amount", "Amount")
	for name, tc := range map[string]struct {
		value  any
		linked bool
	}{
		"text":        {"Salary", true},
		"nil value":   {nil, true},
		"empty text":  {"", true},
		"amount":      {"150.00", true},
		"negative":    {"-5", true},
		"em dash":     {"—", false},
		"hyphen":      {"-", false},
		"zero text":   {"0", false},
		"zero money":  {"0.00", false},
		"zero comma":  {"0,00", false},
		"zero int":    {0, false},
		"zero int64":  {int64(0), false},
		"zero float":  {0.0, false},
		"positive":    {42, true},
		"nbsp padded": {" — ", false},
	} {
		t.Run(name, func(t *testing.T) {
			cell := Cell(templ.Raw("shown"), tc.value, WithCellLink("/record/1"))
			out := renderIn(t, ctx, cell.Component(col, false, true, templ.Attributes{}))
			assert.Equal(t, tc.linked, strings.Contains(out, `href="/record/1"`))
			assert.Contains(t, out, "shown")
		})
	}
}

func TestCellLinkIsStretchedOverTheCell(t *testing.T) {
	ctx := testPageCtx(t, "/transfers")
	cell := Cell(templ.Raw("Salary"), "Salary", WithCellLink("/record/1"))
	out := renderIn(t, ctx, cell.Component(Column("a", "A"), false, true, templ.Attributes{}))
	assert.Contains(t, out, "table-cell-link")
}

func TestPrimaryColumnSkipsPriorityHiddenAndDefaultHiddenColumns(t *testing.T) {
	cfg := NewTableConfig("Employees", "/employees")
	cfg.AddCols(
		Column("id", "ID", WithPriority(3)),
		Column("code", "Code", WithDefaultHidden()),
		Column("name", "Name"),
	)
	assert.Equal(t, MobileRoleHidden, cfg.resolvedMobileRole(cfg.Columns[0]))
	assert.Equal(t, MobileRoleHidden, cfg.resolvedMobileRole(cfg.Columns[1]))
	assert.Equal(t, MobileRolePrimary, cfg.resolvedMobileRole(cfg.Columns[2]))
}

func TestCardLayoutKeepsSortControls(t *testing.T) {
	ctx := testPageCtx(t, "/transfers")
	cfg := NewTableConfig("Transfers", "/transfers")
	cfg.Configurable = false
	cfg.AddCols(
		Column("date", "Date", WithSortable(), WithSortURL("/transfers?sort=date&order=asc"), WithSortDir(SortDirectionAsc)),
		Column("note", "Note"),
	)

	out := renderIn(t, ctx, Table(cfg))
	assert.Contains(t, out, "data-mobile-sort")
	assert.Contains(t, out, `hx-get="/transfers?sort=date&amp;order=asc"`)
	assert.Contains(t, out, `aria-pressed="true"`)

	cfg.MobileLayout = MobileLayoutTable
	assert.NotContains(t, renderIn(t, ctx, Table(cfg)), "data-mobile-sort")
}
