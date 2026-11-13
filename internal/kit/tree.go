package kit

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Branch is one branch of a tree: a heading, and the leaves under it.
type Branch struct {
	Name   string
	Leaves []string
}

// TreeSpec describes what a tree draws and what its clicks mean. Branch and leaf
// indexes are the caller's own, so they stay stable whatever is filtered out.
type TreeSpec struct {
	Branches []Branch
	// Visible filters leaves; nil shows every leaf. A branch with no visible leaf
	// is not drawn at all.
	Visible  func(branch, leaf int) bool
	Open     func(branch int) bool
	OnToggle func(branch int)
	Chosen   func(branch, leaf int) bool
	Expired  func(branch, leaf int) bool
	OnChoose func(branch, leaf int)
	// Actions draw independent controls to the right of the row label.
	BranchAction func(ui.C, gfx.Theme, int)
	LeafAction   func(ui.C, gfx.Theme, int, int)
	// Empty is shown when nothing is visible.
	Empty string
}

// Tree draws a two-level tree: branches that fold, and leaves that select. It
// fills the rest of the column it is declared in.
type Tree struct {
	ScrollID gfx.ID
	BranchID func(branch int) gfx.ID
	LeafID   func(branch, leaf int) gfx.ID
}

// Draw paints the tree into a column that takes the remaining height.
func (t *Tree) Draw(f ui.C, theme gfx.Theme, spec TreeSpec) {
	ui.Scroll(f.Key("skill-list")).Grow(1).FillWidth().PadSides(4, 8, 12, 8).Gap(1).Children(func() {
		drawn := 0
		for branch, node := range spec.Branches {
			if !t.anyVisible(spec, branch) {
				continue
			}
			drawn++
			open := spec.Open(branch)
			marker := "▶"
			if open {
				marker = "▼"
			}
			ui.Row(f).FillWidth().Items(ui.Center).Gap(2).Children(func() {
				row := ui.Row(f).Basis(0).Grow(1).PadSides(6, 0, 6, 8).Radius(6).Items(ui.Center).
					Hover(ui.Look{}.Background(theme.Hover)).Hit(t.BranchID(branch), node.Name)
				if row.Children(func() {
					ui.Text(f, marker).FontSize(8).Muted().Width(14)
					ui.Text(f, node.Name).FontSize(12).Bold().FillWidth().Ellipsis()
				}).Clicked() {
					spec.OnToggle(branch)
				}
				if spec.BranchAction != nil {
					spec.BranchAction(f, theme, branch)
				}
			})
			if !open {
				continue
			}
			for leaf, name := range node.Leaves {
				if spec.Visible != nil && !spec.Visible(branch, leaf) {
					continue
				}
				// The leaves are indented past their branch's marker.
				ui.Row(f).FillWidth().Items(ui.Center).Gap(2).Children(func() {
					item := ui.Row(f).Basis(0).Grow(1).PadSides(5, 0, 5, 30).Radius(6).
						Hover(ui.Look{}.Background(theme.Hover)).Hit(t.LeafID(branch, leaf), name)
					if spec.Chosen != nil && spec.Chosen(branch, leaf) {
						item.Background(theme.Secondary)
					}
					if item.Children(func() {
						label := ui.Text(f, name).FontSize(12).FillWidth().Ellipsis()
						if spec.Expired != nil && spec.Expired(branch, leaf) {
							label.Muted().Label(name).DrawOver(drawExpiredName)
						}
					}).Clicked() {
						spec.OnChoose(branch, leaf)
					}
					if spec.LeafAction != nil {
						spec.LeafAction(f, theme, branch, leaf)
					}
				})
			}
		}
		if drawn == 0 && spec.Empty != "" {
			ui.Text(f, spec.Empty).FontSize(12).Muted().Pad(8)
		}
	})
}

// anyVisible reports whether a branch has at least one visible leaf.
func (t *Tree) anyVisible(spec TreeSpec, branch int) bool {
	if spec.Visible == nil {
		return len(spec.Branches[branch].Leaves) > 0
	}
	for leaf := range spec.Branches[branch].Leaves {
		if spec.Visible(branch, leaf) {
			return true
		}
	}
	return false
}

// drawExpiredName decorates only the visible width of the skill name.
func drawExpiredName(d *ui.DrawContext) {
	width := min(d.Rect.Width, d.Canvas.TextWidth(d.Label, 12))
	d.Canvas.Fill(gfx.Rect{X: d.Rect.X, Y: d.Rect.Y + d.Rect.Height/2, Width: width, Height: 1}, d.Theme.MutedForeground)
}
