// Package pages defines what a page of the window is, so the shell can show one
// without knowing which one it is.
package pages

import (
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// Page is one section of the window: what the navigation selects, and what the
// shell gives room to.
type Page interface {
	// Name is the navigation label.
	Name() string
	// Title is what the content toolbar shows.
	Title() string
	// Pane is the page's own column beside the content, or nil.
	Pane() *Pane
	// Actions are the buttons the content toolbar offers.
	Actions() []Action
	// Fields are the page's text fields, which the shell hands to the platform so
	// typing reaches the focused one.
	Fields() []*flex.Field
	// Content paints the page body in the column the shell opened for it.
	Content(f *flex.Frame, theme ui.Theme)
	// Dialog paints the page's modal over the window, if it has one open.
	Dialog(f *flex.Frame, theme ui.Theme)
	// Wake runs on the window thread before a repaint that background work asked
	// for, and is where a page moves what that work published into its own state.
	Wake()
}

// Pane is a page's own column beside the content. The page owns the width, the
// shell owns the splitter that changes it.
type Pane struct {
	Split *flex.SplitterState
	// Min is the narrowest the column may get.
	Min  float32
	Draw func(f *flex.Frame, theme ui.Theme, width float32)
}

// Action is one button in the content toolbar.
type Action struct {
	ID    ui.ID
	Label string
	// Disabled draws the button unclickable: for what the page cannot do yet, so
	// pressing it is not a press that does nothing.
	Disabled bool
	Run      func()
}

// Base answers for the parts a page does not have, so a page declares only what
// it uses.
type Base struct{}

// Pane reports that the page has no column of its own.
func (Base) Pane() *Pane { return nil }

// Actions reports that the page offers no toolbar buttons.
func (Base) Actions() []Action { return nil }

// Fields reports that the page has no text fields.
func (Base) Fields() []*flex.Field { return nil }

// Dialog reports that the page has no modal.
func (Base) Dialog(*flex.Frame, ui.Theme) {}

// Wake reports that the page has no background work to apply.
func (Base) Wake() {}

// Placeholder is the body a page shows before it has anything to show: a title,
// one line of explanation and an empty card where the content will go.
func Placeholder(f *flex.Frame, theme ui.Theme, title, note string) {
	f.Column().Grow(1).Pad(28).Gap(14).Children(func() {
		f.Text(title).FontSize(22).Bold()
		f.Paragraph(note, 2).FontSize(13).Muted()
		f.Column().Grow(1).Radius(10).Background(theme.Card).Border(1, theme.Border)
	})
}
