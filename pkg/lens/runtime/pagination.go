package runtime

import (
	"net/url"
	"strconv"
	"strings"
)

const (
	TablePaginationPanelQuery = "_lp"
	TablePaginationPageQuery  = "_lpage"
	TablePaginationLimitQuery = "_llimit"

	DefaultTablePage    = 1
	DefaultTablePerPage = 50
)

type TablePagination struct {
	Page    int
	PerPage int
	HasMore bool
}

type TablePageState struct {
	Page    int
	PerPage int
	Offset  int
}

func tableChunkPanel(values url.Values) string {
	return strings.TrimSpace(values.Get(TablePaginationPanelQuery))
}

func IsTableChunkRequest(values url.Values, panelID string) bool {
	panelID = strings.TrimSpace(panelID)
	return panelID != "" && tableChunkPanel(values) == panelID
}

func tablePage(values url.Values, fallback int) int {
	return positiveInt(values.Get(TablePaginationPageQuery), fallback)
}

func tablePerPage(values url.Values, fallback int) int {
	return positiveInt(values.Get(TablePaginationLimitQuery), fallback)
}

func ParseTablePageState(values url.Values, defaultPerPage int) TablePageState {
	perPage := tablePerPage(values, defaultPerPage)
	page := tablePage(values, DefaultTablePage)
	return TablePageState{
		Page:    page,
		PerPage: perPage,
		Offset:  (page - 1) * perPage,
	}
}

func TableChunkScope(values url.Values, panelID string) Scope {
	if IsTableChunkRequest(values, panelID) {
		return PanelScope(panelID)
	}
	return DashboardScope()
}

func ApplyTablePagination(result *Result, panelID string, offset, limit, loadedRows, totalRows int) {
	panel := result.Panel(panelID)
	if panel == nil || limit < 1 {
		return
	}
	page := offset/limit + 1
	if page < DefaultTablePage {
		page = DefaultTablePage
	}
	panel.TablePagination = &TablePagination{
		Page:    page,
		PerPage: limit,
		HasMore: offset+loadedRows < max(totalRows, 0),
	}
}

func positiveInt(raw string, fallback int) int {
	if fallback < 1 {
		fallback = 1
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}
