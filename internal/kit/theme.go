// Package kit holds the parts of the window that carry no business meaning:
// the navigation column, the tree, the theme switch, the splitter geometry and
// the dialog shell. Pages build their screens out of these.
package kit

import "github.com/vivaui/viva/ui"

// The theme toggle's label says what a click will do, which is what a screen
// reader reads out. The glyph is what it shows: the theme the click switches to.
const (
	toLightLabel = "切换到浅色主题"
	toDarkLabel  = "切换到深色主题"

	sunGlyph  = "\u2600" // ☀ a disc with rays
	moonGlyph = "\u263e" // ☾ a crescent
)

// Theme is the application's appearance. A theme is a value chosen at paint
// time, so switching it reopens nothing and changes no window.
type Theme struct{ dark bool }

// Current is the theme to paint with.
func (t *Theme) Current() ui.Theme {
	if t.dark {
		return ui.DarkTheme()
	}
	return ui.LightTheme()
}

// Dark reports the current appearance.
func (t *Theme) Dark() bool { return t.dark }

// Toggle switches between the light and the dark appearance.
func (t *Theme) Toggle() { t.dark = !t.dark }

// Glyph is what the toggle shows: the theme a click switches to. It is text
// rather than a ui.Icon on purpose: viva dev replays the frame through a
// protocol that carries rectangles, images and text but not the triangles a
// path-based icon is drawn with, so an icon would vanish there.
func (t *Theme) Glyph() string {
	if t.dark {
		return sunGlyph
	}
	return moonGlyph
}

// Label is the toggle's accessible name.
func (t *Theme) Label() string {
	if t.dark {
		return toLightLabel
	}
	return toDarkLabel
}
