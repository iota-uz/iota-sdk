package scaffold

import (
	"context"
	"fmt"
	"io"

	"github.com/a-h/templ"

	cscaffold "github.com/iota-uz/iota-sdk/components/scaffold"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

// TableAdapter adapts scaffold.Table to support pagination
type TableAdapter struct {
	Config     *cscaffold.TableConfig
	Data       cscaffold.TableData
	Page       int
	TotalPages int
	PageCtx    *types.PageContext
}

// Render implements templ.Component interface
func (t *TableAdapter) Render(ctx context.Context, w io.Writer) error {
	// First render the main table
	err := cscaffold.Table(t.Config, t.Data).Render(ctx, w)
	if err != nil {
		return err
	}

	// Then potentially render pagination if needed
	if t.Page > 0 && t.TotalPages > 1 {
		// This is a placeholder for pagination - in a real implementation
		// you would use a pagination component
		_, err = fmt.Fprintf(w, "<div class='pagination'>Page %d of %d</div>", t.Page, t.TotalPages)
	}

	return err
}

// ExtendedTable creates a table with pagination support
func ExtendedTable(config *cscaffold.TableConfig, data cscaffold.TableData, page int, totalPages int, pageCtx *types.PageContext) templ.Component {
	return &TableAdapter{
		Config:     config,
		Data:       data,
		Page:       page,
		TotalPages: totalPages,
		PageCtx:    pageCtx,
	}
}

// ContentAdapter adapts scaffold.Content to support search and pagination
type ContentAdapter struct {
	Config     *cscaffold.TableConfig
	Data       cscaffold.TableData
	Search     string
	Page       int
	TotalPages int
	PageCtx    *types.PageContext
}

// Render implements templ.Component interface
func (c *ContentAdapter) Render(ctx context.Context, w io.Writer) error {
	// For now, just use the base Content - in a real implementation
	// you might want to customize this with search and pagination
	return cscaffold.Page(c.Config, c.Data).Render(ctx, w)
}

// ExtendedContent creates a content component with search and pagination
func ExtendedContent(config *cscaffold.TableConfig, data cscaffold.TableData, search string, page int, totalPages int, pageCtx *types.PageContext) templ.Component {
	return &ContentAdapter{
		Config:     config,
		Data:       data,
		Search:     search,
		Page:       page,
		TotalPages: totalPages,
		PageCtx:    pageCtx,
	}
}

// LayoutAdapter adapts a content component with a layout
type LayoutAdapter struct {
	Content templ.Component
	PageCtx *types.PageContext
}

// Render implements templ.Component interface
func (l *LayoutAdapter) Render(ctx context.Context, w io.Writer) error {
	// This is a placeholder for layout - in a real implementation
	// you would use your application's layout system
	return l.Content.Render(ctx, w)
}

// PageWithLayout wraps content with a layout
func PageWithLayout(content templ.Component, pageCtx *types.PageContext) templ.Component {
	return &LayoutAdapter{
		Content: content,
		PageCtx: pageCtx,
	}
}
