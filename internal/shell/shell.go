// Package shell is the window: one identity tree, one theme, the navigation
// column, the panes it splits, and whichever page is selected.
package shell

import (
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"github.com/lizhian/viva-agents-manager/internal/domain"
	"github.com/lizhian/viva-agents-manager/internal/kit"
	"github.com/lizhian/viva-agents-manager/internal/pages"
	"github.com/lizhian/viva-agents-manager/internal/pages/agentsmd"
	"github.com/lizhian/viva-agents-manager/internal/pages/skills"
)

// The content column's narrowest width, with and without a page column beside
// it.
const (
	contentMin       = 420
	contentMinBeside = 320
)

// Shell owns the window: the pages the navigation selects between, the two
// splitters, and the theme.
type Shell struct {
	tree    gfx.Tree
	theme   kit.Theme
	nav     kit.Nav
	pages   []pages.Page
	current int

	split      ui.SplitterState
	natural    float32
	sized      bool
	view       *ui.Page
	paintTheme gfx.Theme

	divider     gfx.ID
	pageDivider gfx.ID
	trailing    float32

	// invalidator wakes the window for work a page finished off the window
	// thread. It owns no goroutine: an application that never installs anything
	// pays nothing for it.
	invalidator app.Invalidator
}

// New builds the window around the store that holds what it manages.
func New(store domain.Store) *Shell { return NewWithOptions(store, agentsmd.Options{}) }

// NewWithOptions injects instruction storage.
func NewWithOptions(store domain.Store, options agentsmd.Options) *Shell {
	s := &Shell{}
	root := s.tree.Feature("shell")
	s.divider = s.tree.ID(root, "divider")
	s.pageDivider = s.tree.ID(root, "divider-page")
	_, s.trailing = app.TitlebarInsets()

	options.Invalidator = &s.invalidator
	options.TitlebarTrailing = s.trailing
	s.pages = []pages.Page{
		skills.New(&s.tree, s.tree.Sub(root, s.tree.Name("skills")), store, &s.invalidator),
		agentsmd.New(&s.tree, s.tree.Sub(root, s.tree.Name("agentsmd")), options),
	}
	s.nav = kit.Nav{
		Items:    pageNames(s.pages),
		ItemID:   s.tree.Sub(root, s.tree.Name("nav")).At,
		ToggleID: s.tree.ID(root, "theme"),
		Theme:    &s.theme,
	}
	s.view = ui.NewPage(ui.PageConfig{Tree: &s.tree, Theme: &s.paintTheme})
	return s
}

// Config describes the window. Keeping it in a function lets a test drive the
// same application without opening anything.
func (s *Shell) Config() app.Config {
	config := app.Config{
		Tree: &s.tree, Title: "Viva Agents Manager",
		Width: 1100, Height: 720, MinWidth: 720, MinHeight: 480,
		// WindowCustomTitlebar keeps the traffic lights but drops the title bar:
		// the page extends the full height behind them and declares where the
		// window is dragged from.
		WindowMode:     app.WindowCustomTitlebar,
		TitlebarHeight: kit.Titlebar,
	}
	config.Paint = s.paint
	config.OnKey = s.onKey
	config.OnScroll = s.onScroll
	config.OnPointer = s.onPointer
	config.OnShortcut = s.onShortcut
	// A page's background work publishes its result, then wakes the window; every
	// page is given the chance to move that result into its state before the
	// paint that reads it.
	config.Invalidator = &s.invalidator
	config.OnWake = s.wake
	// Every page's fields are handed to the platform at once, so typing reaches
	// whichever one has focus.
	fields := ui.Fields(s.fields()...)
	config.TextInput = func(context app.TextInputContext) *app.TextInput {
		if page, ok := s.page().(interface {
			TextInput(app.TextInputContext) *app.TextInput
		}); ok {
			return page.TextInput(context)
		}
		return fields(context)
	}
	config.OnCloseRequest = func(allow func()) {
		for i, page := range s.pages {
			if closer, ok := page.(interface{ RequestClose(func()) }); ok {
				closer.RequestClose(allow)
				if attention, ok := page.(interface{ NeedsAttention() bool }); ok && attention.NeedsAttention() {
					s.current = i
					s.invalidator.Invalidate()
				}
				return
			}
		}
		allow()
	}
	return config
}

// wake gives each page the background work it finished.
func (s *Shell) wake() {
	for _, page := range s.pages {
		page.Wake()
	}
}

// page is the selected page.
func (s *Shell) page() pages.Page { return s.pages[s.current] }

func (s *Shell) paint(canvas *gfx.Canvas, size gfx.Size) {
	theme := s.theme.Current()
	s.fit(canvas)

	bounds := gfx.Rect{Width: size.Width, Height: size.Height}
	canvas.Fill(bounds, theme.Background)
	// Carry the pane colours to the middle of each splitter's strip, so a
	// divider's line falls on the boundary between panes rather than four pixels
	// inside a gutter. Every pane left of the content is a card.
	right := s.split.Pos + kit.SplitterWidth/2
	if pane := s.page().Pane(); pane != nil {
		right = s.split.Pos + kit.SplitterWidth + pane.Split.Pos + kit.SplitterWidth/2
	}
	kit.CardFill(canvas, theme, right, size.Height)

	// The page is laid out first and painted afterwards: what a page draws
	// itself is a canvas component, not a flex node, and it needs the
	// rectangle the layout gave it.
	s.paintTheme = theme
	s.view.Draw(canvas, bounds, s.declareUI)
	s.page().Overlay(canvas, theme)
}

// declare paints the navigation, the panes and the selected page.
func (s *Shell) declare(f ui.C, theme gfx.Theme) {
	{
		ui.Row(f).Fill().Children(func() {
			s.nav.Draw(f, s.split.Pos, s.current, func(index int) {
				if index == s.current {
					return
				}
				if page, ok := s.page().(interface{ BeforeLeave() bool }); ok && !page.BeforeLeave() {
					return
				}
				s.current = index
			})

			// The sidebar may grow, but not shrink past the width its labels need.
			// What follows it is either the content, or one row holding the page's
			// column and the content: a Splitter measures its limits against the
			// next sibling, so that row has to grow for the sidebar to be able to.
			pane := s.page().Pane()
			if pane == nil {
				ui.Splitter(f, &s.split).Mins(s.natural, contentMin)
				s.content(f, theme)
				return
			}
			ui.Splitter(f, &s.split).Mins(s.natural, pane.Min)
			// FillWidth for the same reason as the content column below: the row's
			// preferred width is the sum of its children's, and the page's column
			// would otherwise claim the room the navigation needs.
			ui.Row(f).FillWidth().Grow(1).Children(func() {
				pane.Draw(f, theme, pane.Split.Pos)
				ui.Splitter(f, pane.Split).Mins(pane.Min, contentMinBeside)
				s.content(f, theme)
			})
		})
		s.page().Dialog(f, theme)
	}
}

// content paints the body of the selected page.
func (s *Shell) content(f ui.C, theme gfx.Theme) {
	// FillWidth, not Grow alone: a column's preferred width is the sum of its
	// children's, and one long paragraph would make that wider than the window and
	// shrink the navigation and the page's column to make room for it.
	ui.Column(f).FillWidth().Grow(1).Children(func() {
		ui.Column(f).FillWidth().PadRight(s.trailing).Children(func() { s.page().Header(f, theme) })
		s.page().Content(f, theme)
	})
}

// onScroll routes the wheel to a list under the pointer before the page
// handles its independently painted documents.
func (s *Shell) onScroll(event gfx.ScrollEvent) gfx.EventResult {
	if result := s.view.Wheel(event); result.Handled {
		return result
	}
	return s.page().Scroll(event)
}

// onPointer gives a pointer event to the selected page.
func (s *Shell) onPointer(event gfx.PointerEvent) gfx.EventResult {
	if result := s.view.Pointer(event); result.Handled {
		return result
	}
	return s.page().Pointer(event)
}

// onShortcut gives a chord to the selected page.
func (s *Shell) onShortcut(event gfx.KeyEvent) gfx.EventResult {
	return s.page().Shortcut(event)
}

// onKey gives the arrow keys to a focused splitter.
func (s *Shell) onKey(event gfx.KeyEvent) gfx.EventResult {
	if result := s.view.Key(event); result.Handled {
		return result
	}
	if page, ok := s.page().(interface {
		Key(gfx.KeyEvent) gfx.EventResult
	}); ok {
		if result := page.Key(event); result.Handled {
			return result
		}
	}
	if result := s.split.Key(event); result.Handled {
		return result
	}
	if pane := s.page().Pane(); pane != nil {
		return pane.Split.Key(event)
	}
	return gfx.EventResult{}
}

// fields collects every page's text fields for the platform's text service.
func (s *Shell) fields() []*ui.Field {
	var fields []*ui.Field
	for _, page := range s.pages {
		fields = append(fields, page.Fields()...)
	}
	return fields
}

// fit gives the navigation column the width of its widest label the first time
// the window is painted, which is when text can be measured.
func (s *Shell) fit(canvas *gfx.Canvas) {
	if s.sized {
		return
	}
	s.sized = true
	s.natural = s.nav.Natural(canvas)
	s.split.Pos = s.natural
}

// pageNames is the navigation's labels.
func pageNames(list []pages.Page) []string {
	names := make([]string, len(list))
	for i, page := range list {
		names[i] = page.Name()
	}
	return names
}

func (s *Shell) declareUI(f ui.C) { s.declare(f, s.paintTheme) }
