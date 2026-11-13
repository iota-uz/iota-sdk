package table

import (
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/a-h/templ"
)

// MobileRoleColumn is implemented by columns that declare a card-layout role.
// It is separate from TableColumn so that existing TableColumn implementations
// keep compiling; a column without it has the MobileRoleAuto role.
type MobileRoleColumn interface {
	MobileRole() MobileRole
}

func columnMobileRole(col TableColumn) MobileRole {
	if rc, ok := col.(MobileRoleColumn); ok {
		return rc.MobileRole()
	}
	return MobileRoleAuto
}

// WithMobileRole sets the role the column plays in the card layout below 640px.
func WithMobileRole(role MobileRole) ColumnOpt {
	return func(c *tableColumnImpl) {
		if role.Valid() {
			c.mobileRole = role
		}
	}
}

// WithMobileLayout selects how the table is drawn below 640px.
func WithMobileLayout(layout MobileLayout) TableConfigOpt {
	return func(c *TableConfig) {
		c.MobileLayout = layout
	}
}

// WithMobilePrimaryAction picks which of Actions stays visible as a button on
// small screens; the others move into the "more" menu. Defaults to the first.
func WithMobilePrimaryAction(index int) TableConfigOpt {
	return func(c *TableConfig) {
		c.MobilePrimaryAction = index
	}
}

// WithoutPushURL stops the table from writing its filter, search and sort
// state into the browser address bar. Use it for a table embedded in a page or
// drawer, where its URL must not replace the host's.
func WithoutPushURL() TableConfigOpt {
	return func(c *TableConfig) {
		c.PushURL = "false"
	}
}

// WithPushURL makes the table push url, chosen by the host page, instead of the
// request URL.
func WithPushURL(url string) TableConfigOpt {
	return func(c *TableConfig) {
		c.PushURL = url
	}
}

// ResolvedPushURL is the value of the form's hx-push-url attribute.
func (c *TableConfig) ResolvedPushURL() string {
	if strings.TrimSpace(c.PushURL) == "" {
		return "true"
	}
	return c.PushURL
}

// UsesCards reports whether rows are drawn as cards on narrow screens.
func (c *TableConfig) UsesCards() bool {
	switch c.MobileLayout {
	case MobileLayoutTable:
		return false
	case MobileLayoutCards:
		return true
	case MobileLayoutAuto:
		return !c.Editable.Enabled
	}
	return !c.Editable.Enabled
}

// resolvedMobileRole returns the role a column plays in the card layout.
func (c *TableConfig) resolvedMobileRole(col TableColumn) MobileRole {
	if role := columnMobileRole(col); role != MobileRoleAuto {
		return role
	}
	if isActionsColumn(col) {
		return MobileRoleAuto
	}
	if c.primaryColumnKey() == col.Key() {
		return MobileRolePrimary
	}
	if col.Priority() >= 2 || col.DefaultHidden() {
		return MobileRoleHidden
	}
	return MobileRoleSecondary
}

// isActionsColumn reports whether a column without a declared role holds row
// actions: it is pinned to the right or its key is "actions".
func isActionsColumn(col TableColumn) bool {
	return col.StickyPos().Right() || strings.EqualFold(col.Key(), "actions")
}

// mobileRoleAttr is the data-mobile-role value of a column. A right-pinned
// column without a declared role is reported as "actions": the stylesheet
// hides it only on rows that are clickable themselves.
func (c *TableConfig) mobileRoleAttr(col TableColumn) string {
	role := c.resolvedMobileRole(col)
	if role == MobileRoleAuto {
		return "actions"
	}
	return string(role)
}

// primaryColumnKey is the key of the column that titles the card: the declared
// primary one, else the first visible, unpinned column without another role.
func (c *TableConfig) primaryColumnKey() string {
	for _, col := range c.Columns {
		if columnMobileRole(col) == MobileRolePrimary {
			return col.Key()
		}
	}
	for _, col := range c.Columns {
		if columnMobileRole(col) != MobileRoleAuto || isActionsColumn(col) || !col.StickyPos().Unknown() && !col.StickyPos().None() || col.Priority() >= 2 || col.DefaultHidden() {
			continue
		}
		return col.Key()
	}
	return ""
}

// rowOpensRecord reports whether a click on the row itself leads somewhere:
// it opens a drawer, loads a page or runs a handler.
func rowOpensRecord(attrs templ.Attributes) bool {
	for _, key := range []string{"hx-get", "hx-post", "data-row-drawer", "data-href", "href", "onclick", "@click", "x-on:click"} {
		if v, ok := attrs[key]; ok && v != nil && v != "" && v != false {
			return true
		}
	}
	return false
}

// hasPlaceholderValue reports whether a cell value stands for "nothing": a
// dash or a zero. Such a cell is not worth linking. A nil or empty value says
// nothing about the content, so it keeps its link.
func hasPlaceholderValue(value any) bool {
	switch v := value.(type) {
	case string:
		return v != "" && isPlaceholderText(v)
	case *string:
		return v != nil && *v != "" && isPlaceholderText(*v)
	case int:
		return v == 0
	case int8:
		return v == 0
	case int16:
		return v == 0
	case int32:
		return v == 0
	case int64:
		return v == 0
	case uint:
		return v == 0
	case uint8:
		return v == 0
	case uint16:
		return v == 0
	case uint32:
		return v == 0
	case uint64:
		return v == 0
	case float32:
		return v == 0
	case float64:
		return v == 0
	case interface{ IsZero() bool }:
		return v.IsZero()
	}
	return false
}

func isPlaceholderText(s string) bool {
	s = strings.Trim(strings.TrimSpace(s), "  ")
	switch s {
	case "-", "–", "—", "−", "0":
		return true
	}
	normalized := strings.NewReplacer(",", ".", " ", "", " ", "", " ", "").Replace(s)
	if f, err := strconv.ParseFloat(normalized, 64); err == nil {
		return f == 0 && !math.IsNaN(f)
	}
	return false
}

// hasActiveQuery reports whether the request carries a search term or a filter,
// as opposed to only paging and sorting parameters.
func hasActiveQuery(cfg *TableConfig, query url.Values) bool {
	if strings.TrimSpace(cfg.SearchValue) != "" {
		return true
	}
	for key, values := range query {
		switch key {
		case QueryParamPage, QueryParamLimit, QueryParamSort, QueryParamOrder:
			continue
		}
		for _, v := range values {
			if strings.TrimSpace(v) != "" {
				return true
			}
		}
	}
	return false
}

// formTriggerGuard keeps a filter-builder popover's draft controls from
// resubmitting the form mid-edit, and ignores the silent events that Reset and
// chip removal send before they submit the form once themselves.
const formTriggerGuard = "!target.closest('[data-fb-popover]') && !(detail && detail.silent)"

// ResolvedHxTrigger is the form's hx-trigger. Listeners sit on the form itself
// and see only events bubbling up from its own controls, so a field elsewhere
// on the page (a theme switch, another table's filter) never submits it. The
// "changed" modifier is avoided on purpose: htmx compares the value of the
// listening element, which for a form is always undefined. Text fields use the
// "input" event instead, which fires only when the value actually changes.
// Alpine select models may update hidden query fields in a microtask, hence the
// small delay on select changes.
func (c *TableConfig) ResolvedHxTrigger() string {
	if c.HxTrigger != "" {
		return c.HxTrigger
	}
	g := formTriggerGuard
	return strings.Join([]string{
		"input[" + g + " && target.matches('input:not([type=checkbox]):not([type=radio]):not([type=hidden])')] delay:300ms",
		"input[" + g + " && target.matches('textarea')] delay:300ms",
		"change[" + g + " && target.matches('input:not([type=checkbox])')] delay:500ms",
		"change[" + g + " && target.matches('select')] delay:10ms",
		"change[" + g + " && target.matches('input[type=checkbox]')]",
		"filter-changed",
	}, ", ")
}

// splitMobileActions separates the action that stays a button on small screens
// from the ones folded into the "more" menu.
func (c *TableConfig) splitMobileActions() (templ.Component, []templ.Component) {
	if len(c.Actions) == 0 {
		return nil, nil
	}
	primary := c.MobilePrimaryAction
	if primary < 0 || primary >= len(c.Actions) {
		primary = 0
	}
	more := make([]templ.Component, 0, len(c.Actions)-1)
	for i, action := range c.Actions {
		if i != primary {
			more = append(more, action)
		}
	}
	return c.Actions[primary], more
}

// sortableColumns are the columns a user can sort by. In the card layout the
// header is hidden, so they get their own control (mobileSortBar).
func (c *TableConfig) sortableColumns() []TableColumn {
	var cols []TableColumn
	for _, col := range c.Columns {
		if col.Sortable() && col.SortURL() != "" && col.Label() != "" {
			cols = append(cols, col)
		}
	}
	return cols
}
