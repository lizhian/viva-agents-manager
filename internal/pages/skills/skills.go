// Package skills is the Skills page: the sources skills come from, the skills in
// them, and installing a source from a GitHub repository.
package skills

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/markdown"
	"github.com/vivaui/viva/ui"

	"github.com/lizhian/viva-agents-manager/internal/domain"
	"github.com/lizhian/viva-agents-manager/internal/icons"
	"github.com/lizhian/viva-agents-manager/internal/kit"
	"github.com/lizhian/viva-agents-manager/internal/pages"
)

const (
	// searchInset is the gap above the search box, and the same one on either side
	// of it: the pane's own drag strip is exactly that tall.
	searchInset = 12
	// paneWidth is the column's first width; the splitter fits it afterwards.
	paneWidth = 220
	// paneMin is the narrowest the column may get: wide enough for the search box
	// and for a source's name beside the tree's chevron.
	paneMin = 196
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

// pendingUpdate is what an update goroutine published for the window thread to
// apply.
type pendingUpdate struct {
	// started is false when nothing was published.
	started  bool
	progress domain.UpdateProgress
	done     bool
	results  []domain.UpdateResult
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

	// The skill's own document, and the preview under it. Both draw to the
	// canvas, so both are painted in Overlay rather than declared in Content.
	main    reader
	preview reader
	// previewFile is what the preview shows, relative to the skill's directory,
	// and empty when the preview is closed.
	previewFile  string
	previewSplit flex.SplitterState
	// bodyHeight is how tall the skill's own body was the last time it was
	// painted with no preview under it, so the preview opens at a fair height.
	bodyHeight float32

	// updating is true from the press that starts an update until its results
	// have been applied; updateCancel stops the one in flight.
	updating        bool
	updateStatus    string
	updateFailed    bool
	updateCancelled bool
	updateCancel    context.CancelFunc

	// mu guards what the goroutines write and Wake reads.
	mu            sync.Mutex
	pending       pending
	pendingUpdate pendingUpdate
}

type ids struct {
	search       ui.ID
	repo         ui.ID
	install      ui.ID
	update       ui.ID
	folder       ui.ID
	previewSplit ui.ID
	previewClose ui.ID
}

// New builds the page in its own identity scope, reading the store once. The
// invalidator is how the install goroutine asks the window to repaint.
func New(tree *ui.Tree, scope ui.Scope, store domain.Store, invalidator *app.Invalidator) *Page {
	p := &Page{tree: tree, scope: scope, store: store, invalidator: invalidator}
	p.split.Pos = paneWidth
	p.ids = ids{
		search:       tree.ID(scope, "search"),
		repo:         tree.ID(scope, "repo"),
		install:      tree.ID(scope, "install"),
		update:       tree.ID(scope, "update"),
		folder:       tree.ID(scope, "folder"),
		previewSplit: tree.ID(scope, "preview-split"),
		previewClose: tree.ID(scope, "preview-close"),
	}
	p.main.links = tree.Sub(scope, tree.Name("links"))
	p.preview.links = tree.Sub(scope, tree.Name("preview-links"))
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
			p.load()
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
	p.load()
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

// Fields are the page's text fields: the search box, and the dialog's source.
func (p *Page) Fields() []*flex.Field { return []*flex.Field{&p.search, &p.repo} }

// Content paints what the chosen skill says about itself and reserves the room
// the bodies take. The bodies are painted in Overlay: the markdown view draws to
// the canvas, and its rectangle is only known once the page is laid out.
func (p *Page) Content(f *flex.Frame, theme ui.Theme) {
	p.main.slotted = false
	p.preview.slotted = false
	f.Column().FillWidth().Grow(1).Pad(28).Gap(14).Children(func() {
		switch {
		case p.main.err != nil:
			f.Text("读取技能文档失败").FontSize(22).Bold()
			f.Paragraph(p.main.err.Error(), 3).FontSize(13).Color(theme.Danger)
		case p.chosen.skill == "":
			f.Text(p.Title()).FontSize(22).Bold()
			f.Paragraph("从左边选一个技能。", 2).FontSize(13).Muted()
		default:
			p.drawDocuments(f, theme)
		}
	})
}

// drawDocuments paints the skill's own document and, when it is open, the
// preview under it.
func (p *Page) drawDocuments(f *flex.Frame, theme ui.Theme) {
	if p.previewFile == "" {
		p.main.slot = f.Slot().Grow(1).FillWidth()
		p.main.slotted = true
		return
	}
	p.main.slot = f.Slot().Height(p.previewSplit.Pos).FillWidth()
	p.main.slotted = true
	f.Splitter(p.ids.previewSplit, &p.previewSplit).Mins(previewMin, previewMin)
	f.Column().Grow(1).FillWidth().Gap(8).Children(func() {
		f.Row().FillWidth().Items(flex.Center).Gap(8).Children(func() {
			f.Text(p.preview.document.Name).FontSize(13).Bold().Ellipsis()
			f.Spacer()
			if f.Button(p.ids.previewClose, "关闭").Outline().XSmall().Clicked() {
				p.closePreview()
			}
		})
		if p.preview.err != nil {
			f.Paragraph(p.preview.err.Error(), 3).FontSize(13).Color(theme.Danger)
			return
		}
		p.preview.slot = f.Slot().Grow(1).FillWidth()
		p.preview.slotted = true
	})
}

// Overlay paints the documents into the room Content reserved for them.
func (p *Page) Overlay(canvas *ui.Canvas, theme ui.Theme) {
	if p.main.slotted {
		p.main.paint(canvas, theme)
		if p.previewFile == "" {
			p.bodyHeight = p.main.area.Height
		}
	}
	if p.preview.slotted {
		p.preview.paint(canvas, theme)
	}
	p.followLinks(canvas)
}

// followLinks opens what a reader's link points at: a file of the skill in the
// preview, anything else in the browser.
func (p *Page) followLinks(canvas *ui.Canvas) {
	for _, r := range []*reader{&p.preview, &p.main} {
		target, ok := r.clicked(canvas)
		if !ok {
			continue
		}
		if file, local := domain.LinkTarget(target); local {
			p.openPreview(file)
			return
		}
		_ = app.OpenURL(target)
		return
	}
}

// Scroll moves the document the pointer is over.
func (p *Page) Scroll(event ui.ScrollEvent) ui.EventResult {
	if p.preview.slotted && covers(p.preview.area, event.X, event.Y) {
		return p.preview.scroll.Wheel(event)
	}
	if !p.main.slotted {
		return ui.EventResult{}
	}
	return p.main.scroll.Wheel(event)
}

// Pointer selects the text of a document: a press starts a selection, a drag
// extends it, and a release that chose something swallows the click so a link
// under the pointer does not open.
func (p *Page) Pointer(event ui.PointerEvent) ui.EventResult {
	point := ui.Point{X: event.X, Y: event.Y}
	switch event.Kind {
	case ui.PointerDown:
		r := p.readerAt(event.X, event.Y)
		if r == nil {
			return ui.EventResult{}
		}
		p.clearSelections(r)
		if !r.selectAt(point) {
			return ui.EventResult{}
		}
		return ui.EventResult{Repaint: true}
	case ui.PointerMove:
		r := p.selectingReader()
		if r == nil {
			return ui.EventResult{}
		}
		r.extendTo(point)
		return ui.EventResult{Repaint: true}
	case ui.PointerUp:
		r := p.selectingReader()
		if r == nil {
			return ui.EventResult{}
		}
		r.extendTo(point)
		if r.finish() {
			// A drag is not a click: it must not open the link it ended on.
			return ui.EventResult{Handled: true, Repaint: true}
		}
		return ui.EventResult{Repaint: true}
	}
	return ui.EventResult{}
}

// Shortcut copies the text the reader has chosen.
func (p *Page) Shortcut(event ui.KeyEvent) ui.EventResult {
	if event.Text != "c" || event.Modifiers&(ui.ModifierMeta|ui.ModifierControl) == 0 {
		return ui.EventResult{}
	}
	for _, r := range []*reader{&p.main, &p.preview} {
		if text := r.selected(); text != "" {
			return ui.EventResult{Handled: true, ClipboardText: text}
		}
	}
	return ui.EventResult{}
}

// readerAt is the document under a window point, or nil.
func (p *Page) readerAt(x, y float32) *reader {
	if p.preview.slotted && covers(p.preview.area, x, y) {
		return &p.preview
	}
	if p.main.slotted && covers(p.main.area, x, y) {
		return &p.main
	}
	return nil
}

// selectingReader is the document whose selection is being dragged out.
func (p *Page) selectingReader() *reader {
	for _, r := range []*reader{&p.preview, &p.main} {
		if r.selecting {
			return r
		}
	}
	return nil
}

// clearSelections drops what the other document had chosen.
func (p *Page) clearSelections(keep *reader) {
	for _, r := range []*reader{&p.main, &p.preview} {
		if r == keep {
			continue
		}
		r.selection = components.MarkdownSelection{}
		r.selecting = false
	}
}

// selectAt begins a selection at a window point.
func (r *reader) selectAt(point ui.Point) bool {
	offset, ok := r.view.OffsetAt(point)
	if !ok {
		return false
	}
	r.selection = components.MarkdownSelection{Anchor: offset, Focus: offset}
	r.selecting = true
	return true
}

// extendTo moves the end of the selection to a window point.
func (r *reader) extendTo(point ui.Point) {
	if !r.selecting {
		return
	}
	if offset, ok := r.view.OffsetAt(point); ok {
		r.selection.Focus = offset
	}
}

// finish ends the selection, reporting whether it holds any text.
func (r *reader) finish() bool {
	r.selecting = false
	return r.selection.Anchor != r.selection.Focus
}

// selected is the text the selection holds.
func (r *reader) selected() string {
	if r.selection.Anchor == r.selection.Focus {
		return ""
	}
	return r.view.SelectedText(r.selection)
}

// load reads the chosen skill's own document. It runs when the choice changes and
// after the library is reloaded, so a SKILL.md edited by hand shows up on the
// next click.
func (p *Page) load() {
	p.closePreview()
	if p.chosen.skill == "" {
		p.main = reader{links: p.main.links}
		return
	}
	p.main.read(p.store, p.chosen.source, p.chosen.skill, "SKILL.md", p.chosen.skill)
	p.main.render(true)
}

// openPreview shows a file of the chosen skill under its own document.
func (p *Page) openPreview(file string) {
	if file == "" || file == p.previewFile {
		return
	}
	p.previewFile = file
	p.preview.read(p.store, p.chosen.source, p.chosen.skill, file, path.Base(file))
	p.preview.render(false)
	p.previewSplit.Pos = p.mainTop()
}

// mainTop is how tall the skill's own document stays once the preview opens.
func (p *Page) mainTop() float32 {
	if p.bodyHeight <= previewHeight {
		return previewMin
	}
	return p.bodyHeight - previewHeight
}

// closePreview gives the skill's own document the whole height back.
func (p *Page) closePreview() {
	p.previewFile = ""
	p.preview = reader{links: p.preview.links}
	p.previewSplit.Pos = 0
}

// reader is one document column: what it shows, how far it is scrolled, and the
// view that paints it.
type reader struct {
	document domain.Document
	err      error
	parsed   markdown.Document
	view     components.MarkdownView
	scroll   flex.ScrollState
	links    ui.Scope
	// slot is where the body goes, and slotted says Content reserved it this
	// frame: a Box from an older frame must never be read.
	slot    flex.Box
	slotted bool
	// width is the width the body was last laid out at, height the height it came
	// out at, and area where it was painted.
	width  float32
	height float32
	area   ui.Rect
	// selection is the text the reader has chosen, and selecting says a drag is
	// still choosing it.
	selection components.MarkdownSelection
	selecting bool
}

// read loads one file of the skill into the reader.
func (r *reader) read(store domain.Store, source, skill, file, fallback string) {
	*r = reader{links: r.links}
	document, err := store.Document(source, skill, file)
	if err != nil {
		r.err = err
		return
	}
	if document.Name == "" {
		document.Name = fallback
	}
	r.document = document
}

// render lays the document out as markdown. With heading, what the document
// declares about itself goes on top as its own title and summary, so they scroll
// and select with the body instead of standing still above it.
func (r *reader) render(heading bool) {
	var source strings.Builder
	if heading {
		source.WriteString("# " + r.document.Name + "\n\n")
		if r.document.Description != "" {
			source.WriteString(r.document.Description + "\n\n")
		}
	}
	source.WriteString(r.document.Body)
	r.parsed.Parse(domain.RewriteLinks(source.String()))
}

// paint draws the body into the room Content reserved for it.
func (r *reader) paint(canvas *ui.Canvas, theme ui.Theme) {
	area := r.slot.Rect()
	if area.Width <= 0 || area.Height <= 0 {
		return
	}
	r.area = area
	r.scroll.Viewport = area.Height
	// Laying the body out measures its text, which is the expensive half: it is
	// only worth doing when the width changed.
	if r.width != area.Width {
		r.height, r.width = r.view.Layout(canvas, markdownStyle(), &r.parsed, area.Width), area.Width
	}
	r.scroll.Content = r.height
	if r.scroll.Offset > r.scroll.Max() {
		r.scroll.Offset = r.scroll.Max()
	}
	r.view.Paint(canvas, components.MarkdownPaint{
		Look: components.MarkdownLookFor(theme),
		X:    area.X, Y: area.Y - r.scroll.Offset,
		Clip: area, Links: r.links, Selection: r.selection,
	})
}

// clicked is the destination of the link this paint delivered a click on.
func (r *reader) clicked(canvas *ui.Canvas) (string, bool) {
	index, ok := canvas.ClickedIn(r.links)
	if !ok {
		return "", false
	}
	target := r.view.Link(index)
	return target, target != ""
}

// covers reports whether the point is inside the rectangle.
func covers(area ui.Rect, x, y float32) bool {
	return x >= area.X && x < area.X+area.Width && y >= area.Y && y < area.Y+area.Height
}

// The sizes the documents are set in, and the room the preview takes.
const (
	bodySize      = 13
	bodyLeading   = 19
	previewHeight = 260
	previewMin    = 120
)

// markdownStyle is how a skill's body is typeset.
func markdownStyle() components.MarkdownStyle {
	return components.MarkdownStyle{Size: bodySize, LineHeight: bodyLeading}
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
		// The buttons sit in the strip the content toolbar occupies, so the two
		// columns read as one band. The row is the window's drag region, and the
		// buttons inside it keep their own presses. The strip before them stays
		// the drag region, which is what pushes them to its right edge.
		f.Row().Height(kit.Titlebar).PadSides(0, searchInset, 0, searchInset).
			Items(flex.Center).Gap(6).MovesWindow().Children(func() {
			f.Spacer()
			p.buttons(f, theme)
		})
		if p.updateStatus != "" {
			color := theme.MutedForeground
			if p.updateFailed {
				color = theme.Danger
			}
			f.Column().PadSides(2, searchInset, 2, searchInset).Children(func() {
				f.Paragraph(p.updateStatus, 2).FontSize(11).Color(color)
			})
		}
		f.Column().PadSides(0, searchInset, searchInset, searchInset).Children(func() {
			f.Field(&p.search, p.ids.search, "搜索 Skills", "搜索 skills…").FillWidth()
		})
		p.treeView.Draw(f, theme, p.spec)
	})
}

// buttons draws what can be done to the library as a whole. Each one is a glyph,
// so its name is what a screen reader announces rather than anything on screen.
func (p *Page) buttons(f *flex.Frame, theme ui.Theme) {
	install := kit.IconButtonSpec{ID: p.ids.install, Label: "安装", Icon: icons.Plus, Disabled: p.updating}
	if kit.IconButton(f, theme, install) {
		p.openDialog()
	}
	// While the library updates, the same button stops it: a run that takes half a
	// minute needs a way out.
	update := kit.IconButtonSpec{ID: p.ids.update, Label: "更新", Icon: icons.RefreshCw, Disabled: p.installing}
	if p.updating {
		update.Label, update.Icon = "取消", icons.X
	}
	if kit.IconButton(f, theme, update) {
		if p.updating {
			p.cancelUpdate()
		} else {
			p.startUpdate()
		}
	}
	// Viva cannot reveal a directory yet, so the button is drawn inert rather than
	// pressed to no effect. See domain.ErrUnsupported.
	folder := kit.IconButtonSpec{ID: p.ids.folder, Label: "本地文件夹", Icon: icons.FolderOpen, Disabled: true}
	kit.IconButton(f, theme, folder)
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
	p.updateStatus = ""
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

// startUpdate fetches every source again off the window thread. It reports back
// through Wake like an install does.
func (p *Page) startUpdate() {
	ctx, cancel := context.WithCancel(context.Background())
	p.updateCancel = cancel
	p.updating = true
	p.updateFailed = false
	p.updateCancelled = false
	p.updateStatus = "正在更新…"
	go func() {
		results := p.store.UpdateAll(ctx, func(progress domain.UpdateProgress) {
			p.publishUpdate(pendingUpdate{started: true, progress: progress})
		})
		p.publishUpdate(pendingUpdate{started: true, done: true, results: results})
	}()
}

// publishUpdate hands a result to the window thread and asks for the repaint
// that applies it.
func (p *Page) publishUpdate(result pendingUpdate) {
	p.mu.Lock()
	p.pendingUpdate = result
	p.mu.Unlock()
	if p.invalidator != nil {
		p.invalidator.Invalidate()
	}
}

// cancelUpdate asks the update in flight to stop. The sources it already
// replaced stay replaced.
func (p *Page) cancelUpdate() {
	p.updateCancelled = true
	if p.updateCancel != nil {
		p.updateCancel()
	}
}

// wakeUpdate applies what the update goroutine published.
func (p *Page) wakeUpdate() {
	p.mu.Lock()
	result := p.pendingUpdate
	p.pendingUpdate = pendingUpdate{}
	p.mu.Unlock()
	if !result.started {
		return
	}
	if !result.done {
		p.updateStatus = updateProgressLabel(result.progress)
		return
	}
	p.updating = false
	p.updateCancel = nil
	p.updateStatus, p.updateFailed = updateResultLabel(result.results, p.updateCancelled)
	p.reload()
}

// updateProgressLabel is what the status line says while the library updates.
func updateProgressLabel(progress domain.UpdateProgress) string {
	return fmt.Sprintf("正在更新 %d/%d · %s", progress.Done+1, progress.Total, progress.Source)
}

// updateResultLabel is what the status line says once the update is over, and
// whether it is bad news.
func updateResultLabel(results []domain.UpdateResult, cancelled bool) (string, bool) {
	if len(results) == 0 {
		if cancelled {
			return "已取消", false
		}
		return "没有可更新的来源", false
	}
	failed := 0
	var first domain.UpdateResult
	for _, result := range results {
		if result.Err == nil {
			continue
		}
		if failed == 0 {
			first = result
		}
		failed++
	}
	switch {
	case failed == 0:
		return fmt.Sprintf("%d 个来源已更新", len(results)), false
	case cancelled || errors.Is(first.Err, context.Canceled):
		return fmt.Sprintf("已取消，%d 个来源已更新", len(results)-failed), false
	default:
		message := fmt.Sprintf("%s — %s", first.Source, failureMessage(first.Err))
		return fmt.Sprintf("%d 个来源已更新，%d 个失败：%s", len(results)-failed, failed, message), true
	}
}

// Wake applies whatever the background work published.
func (p *Page) Wake() {
	p.wakeInstall()
	p.wakeUpdate()
}

// wakeInstall applies what the install goroutine published.
func (p *Page) wakeInstall() {
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
		p.installErr = failureMessage(result.err)
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

// failureMessage turns a failed install or update into the sentence the reader
// sees. The copy lives here, not in the domain, so the model stays free of it.
func failureMessage(err error) string {
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
