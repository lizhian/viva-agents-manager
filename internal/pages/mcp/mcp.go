// Package mcp is the MCP page. It is a placeholder until the application reads
// MCP server configuration.
package mcp

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"github.com/lizhian/viva-agents-manager/internal/pages"
)

// Page is the MCP page's state; it has none yet.
type Page struct{ pages.Base }

// New builds the page.
func New() *Page { return &Page{} }

// Name is the navigation label.
func (p *Page) Name() string { return "MCP" }

// Content paints the placeholder body.
func (p *Page) Content(f ui.C, theme gfx.Theme) {
	pages.Placeholder(f, theme, p.Name(), "这一页还没有内容。")
}
