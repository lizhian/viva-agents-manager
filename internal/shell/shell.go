// Package shell is the window: one identity tree, one theme, the navigation
// column, the panes it splits, and whichever page is selected.
package shell

import (
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"

	"github.com/lizhian/viva-agents-manager/internal/domain"
	"github.com/lizhian/viva-agents-manager/internal/kit"
	"github.com/lizhian/viva-agents-manager/internal/pages"
	"github.com/lizhian/viva-agents-manager/internal/pages/agentsmd"
	"github.com/lizhian/viva-agents-manager/internal/pages/mcp"
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
	tree    ui.Tree
	theme   kit.Theme
	nav     kit.Nav
	pages   []pages.Page
	current int

	split   flex.SplitterState
	natural float32
	sized   bool
	view    flex.View

	divider     ui.ID
	pageDivider ui.ID
	trailing    float32

	// invalidator wakes the window for work a page finished off the window
	// thread. It owns no goroutine: an application that never installs anything
	// pays nothing for it.
	invalidator app.Invalidator
}

// New builds the window around the store that holds what it manages.
func New(store domain.Store) *Shell {
	s := &Shell{}
	root := s.tree.Feature("shell")
	s.divider = s.tree.ID(root, "divider")
	s.pageDivider = s.tree.ID(root, "divider-page")
	_, s.trailing = app.TitlebarInsets()

	s.pages = []pages.Page{
		skills.New(&s.tree, s.tree.Sub(root, s.tree.Name("skills")), store, &s.invalidator),
		mcp.New(),
		agentsmd.New(),
	}
	s.nav = kit.Nav{
		Items:    pageNames(s.pages),
		ItemID:   s.tree.Sub(root, s.tree.Name("nav")).At,
		ToggleID: s.tree.ID(root, "theme"),
		Theme:    &s.theme,
	}
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
	// A page's background work publishes its result, then wakes the window; every
	// page is given the chance to move that result into its state before the
	// paint that reads it.
	config.Invalidator = &s.invalidator
	config.OnWake = s.wake
	// Every page's fields are handed to the platform at once, so typing reaches
	// whichever one has focus.
	config.TextInput = flex.Fields(s.fields()...)
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

func (s *Shell) paint(canvas *ui.Canvas, size ui.Size) {
	theme := s.theme.Current()
	s.fit(canvas)

	bounds := ui.Rect{Width: size.Width, Height: size.Height}
	canvas.Fill(bounds, theme.Background)
	// Carry the pane colours to the middle of each splitter's strip, so a
	// divider's line falls on the boundary between panes rather than four pixels
	// inside a gutter. Every pane left of the content is a card.
	right := s.split.Pos + kit.SplitterWidth/2
	if pane := s.page().Pane(); pane != nil {
		right = s.split.Pos + kit.SplitterWidth + pane.Split.Pos + kit.SplitterWidth/2
	}
	kit.CardFill(canvas, theme, right, size.Height)

	s.view.Draw(canvas, theme, bounds, func(f *flex.Frame) {
		f.Row().Fill().Children(func() {
			s.nav.Draw(f, s.split.Pos, s.current, func(index int) { s.current = index })

			// The sidebar may grow, but not shrink past the width its labels need.
			// What follows it is either the content, or one row holding the page's
			// column and the content: a Splitter measures its limits against the
			// next sibling, so that row has to grow for the sidebar to be able to.
			pane := s.page().Pane()
			if pane == nil {
				f.Splitter(s.divider, &s.split).Mins(s.natural, contentMin)
				s.content(f, theme)
				return
			}
			f.Splitter(s.divider, &s.split).Mins(s.natural, pane.Min)
			f.Row().Grow(1).Children(func() {
				pane.Draw(f, theme, pane.Split.Pos)
				f.Splitter(s.pageDivider, pane.Split).Mins(pane.Min, contentMinBeside)
				s.content(f, theme)
			})
		})
		s.page().Dialog(f, theme)
	})
}

// content paints the toolbar and the body of the selected page.
func (s *Shell) content(f *flex.Frame, theme ui.Theme) {
	page := s.page()
	f.Column().Grow(1).Children(func() {
		// Controls inside a move region keep their own presses, so the strip still
		// drags the window.
		f.Row().Height(kit.Titlebar).PadSides(0, 16+s.trailing, 0, 16).
			Items(flex.Center).Gap(6).MovesWindow().Children(func() {
			f.Text(page.Title()).FontSize(12).Muted().Ellipsis()
			f.Spacer()
			for _, action := range page.Actions() {
				button := f.Button(action.ID, action.Label).Outline().XSmall()
				if button.Disabled(action.Disabled).Clicked() {
					action.Run()
				}
			}
		})
		page.Content(f, theme)
	})
}

// onKey gives the arrow keys to a focused splitter.
func (s *Shell) onKey(event ui.KeyEvent) ui.EventResult {
	if result := s.split.Key(event); result.Handled {
		return result
	}
	if pane := s.page().Pane(); pane != nil {
		return pane.Split.Key(event)
	}
	return ui.EventResult{}
}

// fields collects every page's text fields for the platform's text service.
func (s *Shell) fields() []*flex.Field {
	var fields []*flex.Field
	for _, page := range s.pages {
		fields = append(fields, page.Fields()...)
	}
	return fields
}

// fit gives the navigation column the width of its widest label the first time
// the window is painted, which is when text can be measured.
func (s *Shell) fit(canvas *ui.Canvas) {
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
