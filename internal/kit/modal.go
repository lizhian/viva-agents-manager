package kit

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Modal is one dialog: the identities it needs, and whether it is open.
//
// The identities are separate on purpose. A scrim that shares the cancel
// button's identity swallows that button's click, because the scrim covers the
// whole frame and is painted over everything else — the dialog then cannot be
// closed by the button it offers.
type Modal struct {
	Scope   gfx.ID // the focus scope Tab stays inside
	Dismiss gfx.ID // the scrim and Escape
	Close   gfx.ID // the corner button
	Primary gfx.ID // the confirming button
	Cancel  gfx.ID // the dismissing button

	Open bool
}

// NewModal names a dialog inside parent, so two dialogs of two pages never
// collide.
func NewModal(tree *gfx.Tree, parent gfx.Scope, name string) *Modal {
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
	Focus gfx.ID
}

// Draw paints the dialog while it is open. body and footer declare what is
// inside; the footer's buttons should use the modal's own identities. The
// dialog closes itself when the scrim, the corner button or Escape is used.
func (m *Modal) Draw(f ui.C, spec ModalSpec, body, footer func()) {
	if !m.Open {
		return
	}
	dialog := ui.Dialog(f, ui.DialogSpec{
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
