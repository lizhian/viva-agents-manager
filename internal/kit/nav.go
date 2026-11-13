package kit

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Titlebar is the strip the traffic lights sit in and the window is dragged
// from. The page runs the full height behind it, so the window reads as having
// no title bar at all.
const Titlebar = 38

// DragStrip is the band the window is dragged from where a column has no header
// of its own: as tall as the inset around the page's search box, so the two
// columns below it start at the same height.
const DragStrip = 12

// SplitterWidth is the strip a Splitter reserves: it draws its one-pixel line
// down the middle of that strip, so a pane's colour has to reach the middle for
// the line to sit on the boundary between two panes.
const SplitterWidth = 8

// The navigation column's items and the padding around them, which together
// decide how wide the column has to be.
const (
	navFontSize = 13
	navItemPadX = 10 // inside one item, left and right
	navPadX     = 8  // inside the column, left and right
)

// Nav is the window's left column: the pages it lists, and the footer with the
// version and the theme button.
type Nav struct {
	Items    []string
	ItemID   func(index int) gfx.ID
	ToggleID gfx.ID
	Theme    *Theme
}

// Natural is the width that fits the widest item, measured with the canvas so
// the column fits whatever font the platform uses.
func (n *Nav) Natural(canvas *gfx.Canvas) float32 {
	widest := float32(0)
	for _, name := range n.Items {
		widest = max(widest, canvas.TextWidth(name, navFontSize))
	}
	return widest + 2*navItemPadX + 2*navPadX
}

// Draw paints the column at the given width. onSelect runs on the paint that
// delivers a click on an item.
func (n *Nav) Draw(f ui.C, width float32, selected int, onSelect func(index int)) {
	theme := n.Theme.Current()
	ui.Column(f).Width(width).Background(theme.Card).Children(func() {
		// The strip the traffic lights sit in, and the place the window is dragged
		// from. It carries no title: the window has no title bar.
		ui.Row(f).Height(Titlebar).MovesWindow()
		ui.Column(f).Grow(1).PadSides(4, navPadX, 12, navPadX).Gap(2).Children(func() {
			for i, name := range n.Items {
				item := ui.Row(f).PadSides(7, navItemPadX, 7, navItemPadX).Radius(6).
					Hover(ui.Look{}.Background(theme.Hover)).Hit(n.ItemID(i), name)
				if i == selected {
					item.Background(theme.Secondary)
				}
				if item.Children(func() {
					ui.Text(f, name).FontSize(navFontSize).FillWidth().Ellipsis()
				}).Clicked() {
					onSelect(i)
				}
			}
		})
		ui.Row(f).Height(36).PadSides(0, 10, 0, 14).Items(ui.Center).Children(func() {
			ui.Text(f, "v0.1.0").FontSize(11).Muted()
			ui.Spacer(f)
			spec := IconButtonSpec{ID: n.ToggleID, Label: n.Theme.Label(), Icon: n.Theme.Icon()}
			if IconButton(f, theme, spec) {
				n.Theme.Toggle()
			}
		})
	})
}
