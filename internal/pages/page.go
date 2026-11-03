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
	// Pane is the page's own column beside the content, or nil.
	Pane() *Pane
	// Fields are the page's text fields, which the shell hands to the platform so
	// typing reaches the focused one.
	Fields() []*flex.Field
	// Content paints the page body in the column the shell opened for it.
	Content(f *flex.Frame, theme ui.Theme)
	// Dialog paints the page's modal over the window, if it has one open.
	Dialog(f *flex.Frame, theme ui.Theme)
	// Overlay paints what the page draws itself, after the layout its Content
	// declared. A component that draws straight to the canvas is not a flex node,
	// so the rectangle Content reserved for it is only known once the page is laid
	// out; the shell calls this then.
	Overlay(canvas *ui.Canvas, theme ui.Theme)
	// Scroll gives the wheel to the page, so what it draws itself can move.
	Scroll(event ui.ScrollEvent) ui.EventResult
	// Pointer gives a pointer event to the page, so what it draws itself can be
	// selected.
	Pointer(event ui.PointerEvent) ui.EventResult
	// Shortcut gives a Control- or Meta-modified chord to the page, so what it
	// draws itself can be copied.
	Shortcut(event ui.KeyEvent) ui.EventResult
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

// Base answers for the parts a page does not have, so a page declares only what
// it uses.
type Base struct{}

// Pane reports that the page has no column of its own.
func (Base) Pane() *Pane { return nil }

// Fields reports that the page has no text fields.
func (Base) Fields() []*flex.Field { return nil }

// Dialog reports that the page has no modal.
func (Base) Dialog(*flex.Frame, ui.Theme) {}

// Overlay reports that the page draws nothing itself.
func (Base) Overlay(*ui.Canvas, ui.Theme) {}

// Scroll reports that the page has nothing of its own to scroll.
func (Base) Scroll(ui.ScrollEvent) ui.EventResult { return ui.EventResult{} }

// Pointer reports that the page draws nothing that can be selected.
func (Base) Pointer(ui.PointerEvent) ui.EventResult { return ui.EventResult{} }

// Shortcut reports that the page owns no chords.
func (Base) Shortcut(ui.KeyEvent) ui.EventResult { return ui.EventResult{} }

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
