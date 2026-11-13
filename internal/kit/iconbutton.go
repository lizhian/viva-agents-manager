package kit

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// The icon button: a glyph in a square that lights up under the pointer. The
// side is the theme toggle's, so every icon button in the window is the same
// target, and the glyph is the 16 a Lucide icon is drawn at.
const (
	iconButtonSide   = 24
	iconButtonRadius = 6
	iconGlyphSide    = 16
)

// IconButtonSpec is what one icon button shows and does.
type IconButtonSpec struct {
	// ID names the button in the identity tree, and Label is what a screen
	// reader announces: a glyph says nothing on its own, and these buttons carry
	// no text.
	ID    gfx.ID
	Label string
	Icon  gfx.Icon
	// Disabled draws the glyph at half strength and takes away both the hover
	// and the press, keeping the label: a control that cannot be used yet is
	// still announced.
	Disabled bool
}

// IconButton draws one and reports a click on it. Viva's buttons carry text and
// no icon, so an icon button is a row that is its own hit target.
func IconButton(f ui.C, theme gfx.Theme, spec IconButtonSpec) bool {
	row := ui.Row(f).Size(iconButtonSide, iconButtonSide).Radius(iconButtonRadius).Center().
		HitWith(spec.ID, spec.Label, gfx.HitOptions{Cursor: gfx.PointerCursorHand, Disabled: spec.Disabled})
	if !spec.Disabled {
		row = row.Hover(ui.Look{}.Background(theme.Hover))
	}
	pressed := row.Children(func() {
		ui.Icon(f, spec.Icon).Size(iconGlyphSide, iconGlyphSide).Color(spec.tint(theme))
	}).Clicked()
	return pressed && !spec.Disabled
}

// tint is the glyph's colour: the muted foreground, at half strength when the
// button cannot be used, which is how Viva dims a disabled control.
func (spec IconButtonSpec) tint(theme gfx.Theme) gfx.Color {
	color := theme.MutedForeground
	if spec.Disabled {
		color.Alpha *= 0.5
	}
	return color
}
