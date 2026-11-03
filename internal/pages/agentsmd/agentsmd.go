// Package agentsmd is the AGENTS.md page: the instruction documents an agent
// reads. It is a placeholder until the application reads those files.
package agentsmd

import (
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"

	"github.com/lizhian/viva-agents-manager/internal/pages"
)

// Page is the AGENTS.md page's state; it has none yet.
type Page struct{ pages.Base }

// New builds the page.
func New() *Page { return &Page{} }

// Name is the navigation label.
func (p *Page) Name() string { return "AGENTS.md" }

// Content paints the placeholder body.
func (p *Page) Content(f *flex.Frame, theme ui.Theme) {
	pages.Placeholder(f, theme, p.Name(), "这一页还没有内容。")
}
