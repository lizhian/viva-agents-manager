package icons

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// TestTheWindowIconsCompiled checks that every glyph the window draws came out
// of the compiler with Lucide's geometry and with something to paint. A source
// that compiles to no commands draws nothing, and under `viva dev` nothing is
// what a working glyph looks like too, so only a test can tell the two apart.
func TestTheWindowIconsCompiled(t *testing.T) {
	for name, icon := range map[string]gfx.Icon{
		"sun":              Sun,
		"chevrons-down-up": ChevronsDownUp,
		"chevrons-up-down": ChevronsUpDown,
		"moon":             Moon,
		"plus":             Plus,
		"refresh-cw":       RefreshCw,
		"x":                X,
		"folder-open":      FolderOpen,
	} {
		t.Run(name, func(t *testing.T) {
			if icon.ViewBox != 24 {
				t.Errorf("ViewBox = %v, want Lucide's 24", icon.ViewBox)
			}
			if icon.Stroke != 2 {
				t.Errorf("Stroke = %v, want Lucide's 2", icon.Stroke)
			}
			if len(icon.Commands) == 0 {
				t.Fatal("no commands: the glyph would draw nothing")
			}
			// A stroked glyph is tinted with the theme's colour. A filled layer
			// would carry a colour of its own, which none of these has.
			if len(icon.Layers) != 0 {
				t.Errorf("%d filled layers, want none", len(icon.Layers))
			}
		})
	}
}
