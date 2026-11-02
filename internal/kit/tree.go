package kit

import (
	"github.com/vivaui/viva/flex"
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
	OnChoose func(branch, leaf int)
	// Empty is shown when nothing is visible.
	Empty string
}

// Tree draws a two-level tree: branches that fold, and leaves that select. It
// fills the rest of the column it is declared in.
type Tree struct {
	BranchID func(branch int) ui.ID
	LeafID   func(branch, leaf int) ui.ID
}

// Draw paints the tree into a column that takes the remaining height.
func (t *Tree) Draw(f *flex.Frame, theme ui.Theme, spec TreeSpec) {
	f.Column().Grow(1).PadSides(4, 8, 12, 8).Gap(1).Children(func() {
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
			row := f.Row().PadSides(6, 8, 6, 8).Radius(6).Items(flex.Center).
				HoverBackground(theme.Hover).Hit(t.BranchID(branch), node.Name)
			if row.Children(func() {
				f.Text(marker).FontSize(8).Muted().Width(14)
				f.Text(node.Name).FontSize(12).Bold().FillWidth().Ellipsis()
			}).Clicked() {
				spec.OnToggle(branch)
			}
			if !open {
				continue
			}
			for leaf, name := range node.Leaves {
				if spec.Visible != nil && !spec.Visible(branch, leaf) {
					continue
				}
				// The leaves are indented past their branch's marker.
				item := f.Row().PadSides(5, 8, 5, 30).Radius(6).
					HoverBackground(theme.Hover).Hit(t.LeafID(branch, leaf), name)
				if spec.Chosen != nil && spec.Chosen(branch, leaf) {
					item.Background(theme.Secondary)
				}
				if item.Children(func() {
					f.Text(name).FontSize(12).FillWidth().Ellipsis()
				}).Clicked() {
					spec.OnChoose(branch, leaf)
				}
			}
		}
		if drawn == 0 && spec.Empty != "" {
			f.Text(spec.Empty).FontSize(12).Muted().Pad(8)
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
