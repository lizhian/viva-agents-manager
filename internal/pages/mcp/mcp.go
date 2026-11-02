// Package mcp is the MCP page. It is a placeholder until the application reads
// MCP server configuration.
package mcp

import (
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"

	"github.com/lizhian/viva-agents-manager/internal/pages"
)

// Page is the MCP page's state; it has none yet.
type Page struct{ pages.Base }

// New builds the page.
func New() *Page { return &Page{} }

// Name is the navigation label.
func (p *Page) Name() string { return "MCP" }

// Title is what the content toolbar shows.
func (p *Page) Title() string { return p.Name() }

// Content paints the placeholder body.
func (p *Page) Content(f *flex.Frame, theme ui.Theme) {
	pages.Placeholder(f, theme, p.Title(), "这一页还没有内容。")
}
