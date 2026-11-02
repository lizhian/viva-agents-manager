// Package skills is the Skills page: the sources skills come from, the skills in
// them, and installing a source from a GitHub repository.
package skills

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/vivaui/viva/app"
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

// choice is the skill the content column shows. A name alone is not enough: two
// sources can each hold a skill of the same name.
type choice struct {
	source string
	skill  string
}

// exists reports whether the chosen skill is still in the library.
func (c choice) exists(sources []domain.Source) bool {
	for _, source := range sources {
		if source.Name != c.source {
			continue
		}
		for _, skill := range source.Skills {
			if skill.Name == c.skill {
				return true
			}
		}
	}
	return false
}

// firstChoice is the first skill the tree draws, or nothing when the library is
// empty.
func firstChoice(sources []domain.Source) choice {
	for _, source := range sources {
		if len(source.Skills) > 0 {
			return choice{source: source.Name, skill: source.Skills[0].Name}
		}
	}
	return choice{}
}

// pending is what an install goroutine published for the window thread to apply.
type pending struct {
	// started is false when nothing was published, so a wake asked for by another
	// page costs this one nothing.
	started  bool
	progress domain.Progress
	done     bool
	err      error
}

// Page is the Skills page's state: what the store holds, what the search box
// asks for, and what is selected.
type Page struct {
	pages.Base

	tree        *ui.Tree
	scope       ui.Scope
	store       domain.Store
	invalidator *app.Invalidator
	split       flex.SplitterState
	pane        pages.Pane
	ids         ids
	modal       *kit.Modal
	treeView    kit.Tree

	actions   []pages.Action
	sources   []domain.Source
	branches  []kit.Branch
	spec      kit.TreeSpec
	search    flex.Field
	repo      flex.Field
	query     string
	collapsed map[int]bool
	chosen    choice

	// installing is true from the press that starts an install until its result
	// has been applied; cancel stops the one in flight.
	installing bool
	progress   domain.Progress
	installErr string
	cancel     context.CancelFunc

	// mu guards pending, which the install goroutine writes and Wake reads.
	mu      sync.Mutex
	pending pending
}

type ids struct {
	search  ui.ID
	repo    ui.ID
	install ui.ID
	update  ui.ID
	folder  ui.ID
}

// New builds the page in its own identity scope, reading the store once. The
// invalidator is how the install goroutine asks the window to repaint.
func New(tree *ui.Tree, scope ui.Scope, store domain.Store, invalidator *app.Invalidator) *Page {
	p := &Page{tree: tree, scope: scope, store: store, invalidator: invalidator}
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
		{ID: p.ids.install, Label: "安装", Run: p.openDialog},
		// Nothing refreshes an installed source yet.
		{ID: p.ids.update, Label: "更新", Disabled: true, Run: func() {}},
		// Viva cannot reveal a directory yet, so the button is drawn unclickable
		// rather than pressed to no effect. See domain.ErrUnsupported.
		{ID: p.ids.folder, Label: "本地文件夹", Disabled: true, Run: func() { _ = p.store.OpenFolder() }},
	}
	p.modal = kit.NewModal(tree, scope, "install")
	p.pane = pages.Pane{Split: &p.split, Min: paneMin, Draw: p.drawPane}
	// The callbacks are made once: they read the page, so they follow a reload
	// without being rebuilt.
	p.spec = kit.TreeSpec{
		Visible:  func(branch, leaf int) bool { return p.match(branch, leaf) },
		Open:     p.branchOpen,
		OnToggle: p.toggleBranch,
		Chosen: func(branch, leaf int) bool {
			skill := p.sources[branch].Skills[leaf]
			return skill.Name == p.chosen.skill && p.sources[branch].Name == p.chosen.source
		},
		OnChoose: func(branch, leaf int) {
			p.chosen = choice{source: p.sources[branch].Name, skill: p.sources[branch].Skills[leaf].Name}
		},
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
		leaves[i] = p.tree.Sub(p.scope, p.tree.Name(source.Name))
		names := make([]string, len(source.Skills))
		for j, skill := range source.Skills {
			names[j] = skill.Name
		}
		p.branches[i] = kit.Branch{Name: source.Label(), Leaves: names}
	}
	p.treeView = kit.Tree{
		BranchID: branches.At,
		LeafID:   func(branch, leaf int) ui.ID { return leaves[branch].At(leaf) },
	}
	p.spec.Branches = p.branches
	// An empty library and a search that matched nothing are different states,
	// and saying the wrong one sends the reader looking in the wrong place.
	p.spec.Empty = "没有匹配的 skill"
	if len(p.sources) == 0 {
		p.spec.Empty = "还没有安装任何技能来源"
	}
	if !p.chosen.exists(p.sources) {
		p.chosen = firstChoice(p.sources)
	}
}

// Name is the navigation label.
func (p *Page) Name() string { return "Skills" }

// Title is the selected skill, or the page's own name when nothing is selected.
func (p *Page) Title() string {
	if p.chosen.skill != "" {
		return p.chosen.skill
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

// Dialog paints the install dialog while it is open. An install runs for as long
// as the network takes, so the dialog stays open with the stage it has reached
// and the reason it failed, rather than closing and leaving the page silent.
func (p *Page) Dialog(f *flex.Frame, theme ui.Theme) {
	wasOpen := p.modal.Open
	p.modal.Draw(f, kit.ModalSpec{Title: "安装 Skill 来源", Width: 480, Focus: p.ids.repo},
		func() {
			f.Column().FillWidth().Gap(8).Children(func() {
				f.Field(&p.repo, p.ids.repo, "GitHub 仓库", "owner/repo").FillWidth()
				if p.installing {
					f.Text(progressLabel(p.progress)).FontSize(13).Muted()
				} else {
					f.Paragraph("填写 GitHub 仓库，需要本机可运行 npx skills。", 1).
						FontSize(13).Muted()
				}
				if p.installErr != "" {
					f.Text(p.installErr).FontSize(13).Color(theme.Danger)
				}
			})
		},
		func() {
			if f.Button(p.modal.Cancel, "取消").Outline().Clicked() {
				p.dismiss()
			}
			repo := strings.TrimSpace(p.repo.Text())
			if f.Button(p.modal.Primary, "开始安装").
				Disabled(p.installing || !domain.ValidRepo(repo)).Clicked() {
				p.startInstall(repo)
			}
		})
	// The scrim and Escape close the dialog the way the button does, so an
	// install never keeps running behind a dialog the reader has dismissed.
	if wasOpen && !p.modal.Open && p.installing {
		p.cancelInstall()
	}
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

// openDialog shows the dialog with a clean slate.
func (p *Page) openDialog() {
	p.installErr = ""
	p.modal.Open = true
}

// dismiss closes the dialog, stopping an install that is still running.
func (p *Page) dismiss() {
	if p.installing {
		p.cancelInstall()
		return
	}
	p.modal.Open = false
}

// startInstall runs the install off the window thread: fetching a repository
// takes as long as the network does, and the window must keep drawing while it
// waits. What it produces comes back through Wake.
func (p *Page) startInstall(repo string) {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.installing = true
	p.progress = domain.ProgressPreparing
	p.installErr = ""
	go func() {
		err := p.store.Install(ctx, repo, func(progress domain.Progress) {
			p.publish(pending{started: true, progress: progress})
		})
		p.publish(pending{started: true, progress: domain.ProgressDone, done: true, err: err})
	}()
}

// publish hands a result to the window thread and asks for the repaint that
// applies it.
func (p *Page) publish(result pending) {
	p.mu.Lock()
	p.pending = result
	p.mu.Unlock()
	if p.invalidator != nil {
		p.invalidator.Invalidate()
	}
}

// cancelInstall asks the install in flight to stop. It reports back through Wake
// like any other result, so the page has one place where an install ends.
func (p *Page) cancelInstall() {
	if p.cancel != nil {
		p.cancel()
	}
}

// Wake applies whatever the install goroutine published.
func (p *Page) Wake() {
	p.mu.Lock()
	result := p.pending
	p.pending = pending{}
	p.mu.Unlock()
	if !result.started {
		return
	}
	p.progress = result.progress
	if !result.done {
		return
	}
	p.installing = false
	p.cancel = nil
	switch {
	case errors.Is(result.err, context.Canceled):
		// The reader stopped it; there is nothing to report.
		p.modal.Open = false
	case result.err != nil:
		p.installErr = installMessage(result.err)
	default:
		p.modal.Open = false
		p.repo.SetText("")
		p.reload()
	}
}

// progressLabel is what the dialog says while an install runs.
func progressLabel(progress domain.Progress) string {
	switch progress {
	case domain.ProgressPreparing:
		return "正在下载仓库…"
	case domain.ProgressMigrating:
		return "正在迁移到技能库…"
	default:
		return "正在完成…"
	}
}

// installMessage turns a failed install into the sentence the dialog shows. The
// copy lives here, not in the domain, so the model stays free of it.
func installMessage(err error) string {
	switch {
	case errors.Is(err, domain.ErrEmptyRepo):
		return "请填写仓库"
	case errors.Is(err, domain.ErrBadRepo):
		return "仓库格式应为 owner/repo"
	case errors.Is(err, domain.ErrAlreadyInstalled):
		return "这个仓库已经装过了"
	case errors.Is(err, domain.ErrNoSkills):
		return "这个仓库里没有可安装的技能"
	case errors.Is(err, domain.ErrNpxMissing):
		return "未找到 npx，请安装 Node.js 并检查 PATH"
	default:
		return err.Error()
	}
}
