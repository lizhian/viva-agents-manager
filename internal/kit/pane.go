package kit

import "github.com/vivaui/viva/ui"

// CardFill paints the card colour from the window's left edge to the middle of
// the last card-coloured pane. A Splitter draws its line down the middle of its
// strip, so without this the line would sit four pixels inside a gutter instead
// of on the boundary between two panes.
func CardFill(canvas *ui.Canvas, theme ui.Theme, right, height float32) {
	canvas.Fill(ui.Rect{Width: right, Height: height}, theme.Card)
}
