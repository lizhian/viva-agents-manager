// Package kit holds the parts of the window that carry no business meaning:
// the navigation column, the tree, the theme switch, the splitter geometry and
// the dialog shell. Pages build their screens out of these.
package kit

import (
	"github.com/vivaui/viva/gfx"

	"github.com/lizhian/viva-agents-manager/internal/icons"
)

// The theme toggle's label says what a click will do, which is what a screen
// reader reads out. The glyph is what it shows: the theme the click switches to.
const (
	toLightLabel = "切换到浅色主题"
	toDarkLabel  = "切换到深色主题"
)

// Theme is the application's appearance. A theme is a value chosen at paint
// time, so switching it reopens nothing and changes no window.
type Theme struct{ dark bool }

// Current is the theme to paint with.
func (t *Theme) Current() gfx.Theme {
	if t.dark {
		return gfx.DarkTheme()
	}
	return gfx.LightTheme()
}

// Dark reports the current appearance.
func (t *Theme) Dark() bool { return t.dark }

// Toggle switches between the light and the dark appearance.
func (t *Theme) Toggle() { t.dark = !t.dark }

// Icon is what the toggle shows: the theme a click switches to. It is drawn as
// triangles, which `viva dev` drops when it replays a frame, so the glyph is
// blank there while the button stays announced and clickable; `go run` and a
// built program draw it.
func (t *Theme) Icon() gfx.Icon {
	if t.dark {
		return icons.Sun
	}
	return icons.Moon
}

// Label is the toggle's accessible name.
func (t *Theme) Label() string {
	if t.dark {
		return toLightLabel
	}
	return toDarkLabel
}
