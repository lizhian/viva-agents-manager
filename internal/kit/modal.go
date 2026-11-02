package kit

import (
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// Modal is one dialog: the identities it needs, and whether it is open.
//
// The identities are separate on purpose. A scrim that shares the cancel
// button's identity swallows that button's click, because the scrim covers the
// whole frame and is painted over everything else — the dialog then cannot be
// closed by the button it offers.
type Modal struct {
	Scope   ui.ID // the focus scope Tab stays inside
	Dismiss ui.ID // the scrim and Escape
	Close   ui.ID // the corner button
	Primary ui.ID // the confirming button
	Cancel  ui.ID // the dismissing button

	Open bool
}

// NewModal names a dialog inside parent, so two dialogs of two pages never
// collide.
func NewModal(tree *ui.Tree, parent ui.Scope, name string) *Modal {
	scope := tree.Sub(parent, tree.Name(name))
	return &Modal{
		Scope:   tree.ID(scope, "scope"),
		Dismiss: tree.ID(scope, "dismiss"),
		Close:   tree.ID(scope, "close"),
		Primary: tree.ID(scope, "primary"),
		Cancel:  tree.ID(scope, "cancel"),
	}
}

// ModalSpec is what a dialog says. Focus, when set, takes the keyboard as the
// dialog opens.
type ModalSpec struct {
	Title string
	Width float32
	Focus ui.ID
}

// Draw paints the dialog while it is open. body and footer declare what is
// inside; the footer's buttons should use the modal's own identities. The
// dialog closes itself when the scrim, the corner button or Escape is used.
func (m *Modal) Draw(f *flex.Frame, spec ModalSpec, body, footer func()) {
	if !m.Open {
		return
	}
	dialog := f.Dialog(flex.DialogSpec{
		Title:          spec.Title,
		Width:          spec.Width,
		Scope:          m.Scope,
		DismissID:      m.Dismiss,
		CloseID:        m.Close,
		InitialFocusID: spec.Focus,
	})
	if body != nil {
		dialog.Body(body)
	}
	if footer != nil {
		dialog.Footer(footer)
	}
	if dialog.Dismissed() {
		m.Open = false
	}
}
