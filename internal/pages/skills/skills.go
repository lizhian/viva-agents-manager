// Package skills is the Skills page: the sources skills come from, the skills in
// them, and installing a source from a GitHub repository.
package skills

import (
	"strings"

	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"

	"github.com/lizhian/viva-agents-manager/internal/domain"
	"github.com/lizhian/viva-agents-manager/internal/kit"
	"github.com/lizhian/viva-agents-manager/internal/pages"
)

const (
	// searchInset is the gap above the search box, and the same one on either side
	// of it: the pane's own drag strip is exactly that tall.
	searchInset = 12
	// paneWidth is the column's first width; the splitter fits it afterwards.
	paneWidth = 220
	// paneMin is the narrowest the column may get.
	paneMin = 160
)

// Page is the Skills page's state: what the store holds, what the search box
// asks for, and what is selected.
type Page struct {
	pages.Base

	tree     *ui.Tree
	scope    ui.Scope
	store    domain.Store
	split    flex.SplitterState
	pane     pages.Pane
	ids      ids
	modal    *kit.Modal
	treeView kit.Tree

	actions   []pages.Action
	sources   []domain.Source
	branches  []kit.Branch
	spec      kit.TreeSpec
	search    flex.Field
	repo      flex.Field
	query     string
	collapsed map[int]bool
	chosen    string
}

type ids struct {
	search  ui.ID
	repo    ui.ID
	install ui.ID
	update  ui.ID
	folder  ui.ID
}

// New builds the page in its own identity scope, reading the store once.
func New(tree *ui.Tree, scope ui.Scope, store domain.Store) *Page {
	p := &Page{tree: tree, scope: scope, store: store}
	p.split.Pos = paneWidth
	p.ids = ids{
		search:  tree.ID(scope, "search"),
		repo:    tree.ID(scope, "repo"),
		install: tree.ID(scope, "install"),
		update:  tree.ID(scope, "update"),
		folder:  tree.ID(scope, "folder"),
	}
	// The actions are made once, so a paint does not rebuild their closures.
	p.actions = []pages.Action{
		{ID: p.ids.install, Label: "安装", Run: func() { p.modal.Open = true }},
		{ID: p.ids.update, Label: "更新", Run: func() { p.store.UpdateAll() }},
		{ID: p.ids.folder, Label: "本地文件夹", Run: func() { p.store.OpenFolder() }},
	}
	p.modal = kit.NewModal(tree, scope, "install")
	p.pane = pages.Pane{Split: &p.split, Min: paneMin, Draw: p.drawPane}
	// The callbacks are made once: they read the page, so they follow a reload
	// without being rebuilt.
	p.spec = kit.TreeSpec{
		Visible:  func(branch, leaf int) bool { return p.match(branch, leaf) },
		Open:     p.branchOpen,
		OnToggle: p.toggleBranch,
		Chosen:   func(branch, leaf int) bool { return p.sources[branch].Skills[leaf].Name == p.chosen },
		OnChoose: func(branch, leaf int) { p.chosen = p.sources[branch].Skills[leaf].Name },
		Empty:    "没有匹配的 skill",
	}
	p.reload()
	return p
}

// reload reads the store again and rebuilds the names the tree draws.
func (p *Page) reload() {
	p.sources = p.store.Sources()
	branches := p.tree.Sub(p.scope, p.tree.Name("branches"))
	leaves := make([]ui.Scope, len(p.sources))
	p.branches = make([]kit.Branch, len(p.sources))
	for i, source := range p.sources {
		leaves[i] = p.tree.Sub(p.scope, p.tree.Name(source.Label))
		names := make([]string, len(source.Skills))
		for j, skill := range source.Skills {
			names[j] = skill.Name
		}
		p.branches[i] = kit.Branch{Name: source.Label, Leaves: names}
	}
	p.treeView = kit.Tree{
		BranchID: branches.At,
		LeafID:   func(branch, leaf int) ui.ID { return leaves[branch].At(leaf) },
	}
	p.spec.Branches = p.branches
	if p.chosen == "" && len(p.sources) > 0 && len(p.sources[0].Skills) > 0 {
		p.chosen = p.sources[0].Skills[0].Name
	}
}

// Name is the navigation label.
func (p *Page) Name() string { return "Skills" }

// Title is the selected skill, or the page's own name when nothing is selected.
func (p *Page) Title() string {
	if p.chosen != "" {
		return p.chosen
	}
	return p.Name()
}

// Pane is the page's own column: a search box over the tree of sources.
func (p *Page) Pane() *pages.Pane { return &p.pane }

// Actions are the buttons the toolbar offers.
func (p *Page) Actions() []pages.Action { return p.actions }

// Fields are the page's text fields: the search box, and the dialog's source.
func (p *Page) Fields() []*flex.Field { return []*flex.Field{&p.search, &p.repo} }

// Content paints the selected skill in the content column.
func (p *Page) Content(f *flex.Frame, theme ui.Theme) {
	f.Column().Grow(1).Pad(28).Gap(14).Children(func() {
		f.Text(p.Title()).FontSize(22).Bold()
		f.Paragraph("拖动中间的分栏线调整宽度，顶部空白处可以拖动窗口。", 2).FontSize(13).Muted()
		f.Column().Grow(1).Radius(10).Background(theme.Card).Border(1, theme.Border)
	})
}

// Dialog paints the install dialog while it is open.
func (p *Page) Dialog(f *flex.Frame, theme ui.Theme) {
	p.modal.Draw(f, kit.ModalSpec{Title: "安装 Skill 来源", Width: 480, Focus: p.ids.repo},
		func() {
			f.Column().FillWidth().Gap(8).Children(func() {
				f.Field(&p.repo, p.ids.repo, "GitHub 仓库", "owner/repo").FillWidth()
				f.Paragraph("填写 GitHub 仓库，需要本机可运行 npx skills。", 1).
					FontSize(13).Muted()
			})
		},
		func() {
			if f.Button(p.modal.Cancel, "取消").Outline().Clicked() {
				p.modal.Open = false
			}
			repo := strings.TrimSpace(p.repo.Text())
			if f.Button(p.modal.Primary, "开始安装").Disabled(repo == "").Clicked() {
				if p.store.Install(repo) == nil {
					p.reload()
					p.modal.Open = false
				}
			}
		})
}

// drawPane paints the page's column at the width the splitter gives it.
func (p *Page) drawPane(f *flex.Frame, theme ui.Theme, width float32) {
	p.filter()
	f.Column().Width(width).Background(theme.Card).Children(func() {
		// A strip as tall as the inset around the search box, so the box sits the
		// same distance from the top as from either side, and the window still
		// moves from the band above it.
		f.Row().Height(searchInset).MovesWindow()
		f.Column().PadSides(0, searchInset, searchInset, searchInset).Children(func() {
			f.Field(&p.search, p.ids.search, "搜索 Skills", "搜索 skills…").FillWidth()
		})
		p.treeView.Draw(f, theme, p.spec)
	})
}

// filter keeps the query in step with the search box. The text is read and
// compared once per change, so a frame that changes nothing costs nothing.
func (p *Page) filter() {
	query := strings.ToLower(strings.TrimSpace(p.search.Text()))
	if query != p.query {
		p.query = query
	}
}

// match reports whether a leaf survives the search.
func (p *Page) match(branch, leaf int) bool {
	if p.query == "" {
		return true
	}
	return p.sources[branch].Skills[leaf].Match(p.query)
}

// branchOpen reports whether a branch's leaves are drawn: a search opens every
// branch that still has a match.
func (p *Page) branchOpen(branch int) bool {
	return p.query != "" || !p.collapsed[branch]
}

func (p *Page) toggleBranch(branch int) {
	if p.collapsed == nil {
		p.collapsed = map[int]bool{}
	}
	p.collapsed[branch] = !p.collapsed[branch]
}
