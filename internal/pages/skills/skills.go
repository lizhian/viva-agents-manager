// Package skills is the Skills page: the sources skills come from, the skills in
// them, and installing a source from a GitHub repository.
package skills

import (
	"context"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
	"sync"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
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

	tree        *gfx.Tree
	scope       gfx.Scope
	store       domain.Store
	invalidator *app.Invalidator
	split       ui.SplitterState
	pane        pages.Pane
	ids         ids
	modal       *kit.Modal
	treeView    kit.Tree

	sources            []domain.Source
	branches           []kit.Branch
	spec               kit.TreeSpec
	search             ui.Field
	repo               ui.Field
	query              string
	collapsed          map[int]bool
	foldOverride       bool
	openingFolder      bool
	folderDone         bool  // guarded by mu
	folderErr          error // guarded by mu
	sourceSwitchValues []bool
	branchSwitchLabels []string
	leafSwitchLabels   [][]string
	branchSwitches     gfx.Scope
	leafSwitches       []gfx.Scope
	chosen             choice
	characterLabel     string
	expirySlot         ui.Box
	expiryVisible      bool

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
	previewSplit ui.SplitterState
	// bodyHeight is how tall the skill's own body was the last time it was
	// painted with no preview under it, so the preview opens at a fair height.
	bodyHeight float32

	// updating is true from the press that starts an update until its results
	// have been applied; updateCancel stops the one in flight.
	updating        bool
	enabledSummary  string
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
	search       gfx.ID
	repo         gfx.ID
	install      gfx.ID
	update       gfx.ID
	folder       gfx.ID
	skillFolder  gfx.ID
	fold         gfx.ID
	previewSplit gfx.ID
	previewClose gfx.ID
	expired      gfx.ID
}

// New builds the page in its own identity scope, reading the store once. The
// invalidator is how the install goroutine asks the window to repaint.
func New(tree *gfx.Tree, scope gfx.Scope, store domain.Store, invalidator *app.Invalidator) *Page {
	p := &Page{tree: tree, scope: scope, store: store, invalidator: invalidator}
	p.split.Pos = paneWidth
	p.ids = ids{
		search:       tree.ID(scope, "search"),
		expired:      tree.ID(scope, "expired"),
		repo:         tree.ID(scope, "repo"),
		install:      tree.ID(scope, "install"),
		update:       tree.ID(scope, "update"),
		folder:       tree.ID(scope, "folder"),
		skillFolder:  tree.ID(scope, "skill-folder"),
		fold:         tree.ID(scope, "fold"),
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
		Visible:      func(branch, leaf int) bool { return p.match(branch, leaf) },
		Open:         p.branchOpen,
		OnToggle:     p.toggleBranch,
		BranchAction: p.drawSourceSwitch,
		LeafAction:   p.drawSkillSwitch,
		Expired:      func(branch, leaf int) bool { return p.sources[branch].Skills[leaf].Expired },
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
	p.reloadSources()
	p.load()
}

func (p *Page) reloadSources() {
	p.sources = slices.Clone(p.store.Sources())
	enabled := 0
	for _, source := range p.sources {
		for _, skill := range source.Skills {
			if skill.Enabled && !skill.Expired {
				enabled++
			}
		}
	}
	p.enabledSummary = fmt.Sprintf("已启用 %d 个技能", enabled)
	for i := range p.sources {
		p.sources[i].Skills = slices.Clone(p.sources[i].Skills)
		slices.SortFunc(p.sources[i].Skills, compareSkills)
	}
	branches := p.tree.Sub(p.scope, p.tree.Name("branches"))
	leaves := make([]gfx.Scope, len(p.sources))
	p.branchSwitches = p.tree.Sub(p.scope, p.tree.Name("source-switches"))
	p.leafSwitches = make([]gfx.Scope, len(p.sources))
	p.branchSwitchLabels = make([]string, len(p.sources))
	p.sourceSwitchValues = make([]bool, len(p.sources))
	p.leafSwitchLabels = make([][]string, len(p.sources))
	p.branches = make([]kit.Branch, len(p.sources))
	for i, source := range p.sources {
		leaves[i] = p.tree.Sub(p.scope, p.tree.Name(source.Name))
		p.leafSwitches[i] = p.tree.Sub(p.branchSwitches, p.tree.Name(source.Name))
		p.branchSwitchLabels[i] = "启用来源 " + source.Label()
		p.leafSwitchLabels[i] = make([]string, len(source.Skills))
		names := make([]string, len(source.Skills))
		for j, skill := range source.Skills {
			names[j] = skill.Name
			p.leafSwitchLabels[i][j] = "启用技能 " + source.Label() + "/" + skill.Name
		}
		p.branches[i] = kit.Branch{Name: source.Label(), Leaves: names}
	}
	p.treeView = kit.Tree{
		ScrollID: p.tree.ID(p.scope, "list-scroll"),
		BranchID: branches.At,
		LeafID:   func(branch, leaf int) gfx.ID { return leaves[branch].At(leaf) },
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

// Fields are the page's text fields: the search box, and the dialog's source.
func (p *Page) Fields() []*ui.Field { return []*ui.Field{&p.search, &p.repo} }

// Content paints what the chosen skill says about itself and reserves the room
// the bodies take. The bodies are painted in Overlay: the markdown view draws to
// the canvas, and its rectangle is only known once the page is laid out.
func (p *Page) Content(f ui.C, theme gfx.Theme) {
	p.main.slotted = false
	p.preview.slotted = false
	ui.Column(f).FillWidth().Grow(1).PadSides(8, 28, 28, 28).Gap(14).Children(func() {
		switch {
		case p.main.err != nil:
			ui.Text(f, "读取技能文档失败").FontSize(22).Bold()
			ui.Paragraph(f, p.main.err.Error(), 3).FontSize(13).Color(theme.Danger)
		case p.chosen.skill == "":
			ui.Text(f, p.Title()).FontSize(22).Bold()
			ui.Paragraph(f, "从左边选一个技能。", 2).FontSize(13).Muted()
		default:
			p.drawDocuments(f, theme)
		}
	})
}

// drawDocuments paints the skill's own document and, when it is open, the
// preview under it.
func (p *Page) drawDocuments(f ui.C, theme gfx.Theme) {
	if p.previewFile == "" {
		p.main.slot = ui.Slot(f).Grow(1).FillWidth()
		p.main.slotted = true
		return
	}
	p.main.slot = ui.Slot(f).Height(p.previewSplit.Pos).FillWidth()
	p.main.slotted = true
	ui.Splitter(f, &p.previewSplit).Mins(previewMin, previewMin)
	ui.Column(f).Grow(1).FillWidth().Gap(8).Children(func() {
		ui.Row(f).FillWidth().Items(ui.Center).Gap(8).Children(func() {
			ui.Text(f, p.preview.document.Name).FontSize(13).Bold().Ellipsis()
			ui.Spacer(f)
			if ui.Button(f, "关闭").Outline().XSmall().Clicked() {
				p.closePreview()
			}
		})
		if p.preview.err != nil {
			ui.Paragraph(f, p.preview.err.Error(), 3).FontSize(13).Color(theme.Danger)
			return
		}
		p.preview.slot = ui.Slot(f).Grow(1).FillWidth()
		p.preview.slotted = true
	})
}

// Overlay paints the documents into the room Content reserved for them.
func (p *Page) Overlay(canvas *gfx.Canvas, theme gfx.Theme) {
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
	p.paintExpiry(canvas, theme)
}

// followLinks opens what a reader's link points at: a file of the skill in the
// preview, anything else in the browser.
func (p *Page) followLinks(canvas *gfx.Canvas) {
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
func (p *Page) Scroll(event gfx.ScrollEvent) gfx.EventResult {
	if p.preview.slotted && covers(p.preview.area, event.X, event.Y) {
		return p.preview.scroll.Wheel(event)
	}
	if !p.main.slotted || !covers(p.main.area, event.X, event.Y) {
		return gfx.EventResult{}
	}
	return p.main.scroll.Wheel(event)
}

// Pointer selects the text of a document: a press starts a selection, a drag
// extends it, and a release that chose something swallows the click so a link
// under the pointer does not open.
func (p *Page) Pointer(event gfx.PointerEvent) gfx.EventResult {
	point := gfx.Point{X: event.X, Y: event.Y}
	switch event.Kind {
	case gfx.PointerDown:
		r := p.readerAt(event.X, event.Y)
		if r == nil {
			return gfx.EventResult{}
		}
		p.clearSelections(r)
		if !r.selectAt(point) {
			return gfx.EventResult{}
		}
		return gfx.EventResult{Repaint: true}
	case gfx.PointerMove:
		r := p.selectingReader()
		if r == nil {
			return gfx.EventResult{}
		}
		r.extendTo(point)
		return gfx.EventResult{Repaint: true}
	case gfx.PointerUp:
		r := p.selectingReader()
		if r == nil {
			return gfx.EventResult{}
		}
		r.extendTo(point)
		if r.finish() {
			// A drag is not a click: it must not open the link it ended on.
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		return gfx.EventResult{Repaint: true}
	}
	return gfx.EventResult{}
}

// Shortcut copies the text the reader has chosen.
func (p *Page) Shortcut(event gfx.KeyEvent) gfx.EventResult {
	if event.Text != "c" || event.Modifiers&(gfx.ModifierMeta|gfx.ModifierControl) == 0 {
		return gfx.EventResult{}
	}
	for _, r := range []*reader{&p.main, &p.preview} {
		if text := r.selected(); text != "" {
			return gfx.EventResult{Handled: true, ClipboardText: text}
		}
	}
	return gfx.EventResult{}
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
func (r *reader) selectAt(point gfx.Point) bool {
	offset, ok := r.view.OffsetAt(point)
	if !ok {
		return false
	}
	r.selection = components.MarkdownSelection{Anchor: offset, Focus: offset}
	r.selecting = true
	return true
}

// extendTo moves the end of the selection to a window point.
func (r *reader) extendTo(point gfx.Point) {
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
	p.characterLabel = fmt.Sprintf("总字数 %d", p.main.document.CharacterCount)
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
	scroll   ui.ScrollState
	links    gfx.Scope
	// slot is where the body goes, and slotted says Content reserved it this
	// frame: a Box from an older frame must never be read.
	slot    ui.Box
	slotted bool
	// width is the width the body was last laid out at, height the height it came
	// out at, and area where it was painted.
	width  float32
	height float32
	area   gfx.Rect
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
func (r *reader) paint(canvas *gfx.Canvas, theme gfx.Theme) {
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
func (r *reader) clicked(canvas *gfx.Canvas) (string, bool) {
	index, ok := canvas.ClickedIn(r.links)
	if !ok {
		return "", false
	}
	target := r.view.Link(index)
	return target, target != ""
}

// covers reports whether the point is inside the rectangle.
func covers(area gfx.Rect, x, y float32) bool {
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
func (p *Page) Dialog(f ui.C, theme gfx.Theme) {
	wasOpen := p.modal.Open
	p.modal.Draw(f, kit.ModalSpec{Title: "安装 Skill 来源", Width: 480, Focus: p.ids.repo},
		func() {
			ui.Column(f).FillWidth().Gap(8).Children(func() {
				ui.TextFieldID(f, p.ids.repo, &p.repo, "GitHub 仓库", "owner/repo").FillWidth()
				if p.installing {
					ui.Text(f, progressLabel(p.progress)).FontSize(13).Muted()
				} else {
					ui.Paragraph(f, "填写 owner/repo，需要本机可运行 npx skills。@ 为保留分隔符。", 3).
						FontSize(13).Muted()
				}
				if p.installErr != "" {
					ui.Text(f, p.installErr).FontSize(13).Color(theme.Danger)
				}
			})
		},
		func() {
			if ui.Button(f, "取消").Outline().Clicked() {
				p.dismiss()
			}
			repo := strings.TrimSpace(p.repo.Text())
			if ui.Button(f, "开始安装").
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
func (p *Page) drawPane(f ui.C, theme gfx.Theme, width float32) {
	p.filter()
	ui.Column(f).Width(width).Background(theme.Card).Children(func() {
		// The buttons sit in the strip the content toolbar occupies, so the two
		// columns read as one band. The row is the window's drag region, and the
		// buttons inside it keep their own presses. Status shares the strip with
		// them so updates never shift the search box or tree below it.
		ui.Row(f).Height(kit.Titlebar).PadSides(0, searchInset, 0, searchInset).
			Items(ui.Center).Gap(6).MovesWindow().Children(func() {
			if p.updateStatus != "" {
				color := theme.MutedForeground
				if p.updateFailed {
					color = theme.Danger
				}
				ui.Text(f, p.updateStatus).FontSize(11).Color(color).
					Basis(0).Grow(1).Ellipsis()
			} else {
				ui.Text(f, p.enabledSummary).FontSize(11).Muted().
					Basis(0).Grow(1).Ellipsis()
			}
			p.buttons(f, theme)
		})
		ui.Column(f).PadSides(0, searchInset, searchInset, searchInset).Children(func() {
			ui.TextFieldID(f, p.ids.search, &p.search, "搜索 Skills", "搜索 skills…").FillWidth()
		})
		p.treeView.Draw(f, theme, p.spec)
	})
}

// buttons draws what can be done to the library as a whole. Each one is a glyph,
// so its name is what a screen reader announces rather than anything on screen.
func (p *Page) buttons(f ui.C, theme gfx.Theme) {
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
	folder := kit.IconButtonSpec{ID: p.ids.folder, Label: "本地文件夹", Icon: icons.FolderOpen, Disabled: p.openingFolder}
	if kit.IconButton(f, theme, folder) {
		p.openFolder(false)
	}
	fold := kit.IconButtonSpec{ID: p.ids.fold, Label: "全部展开", Icon: icons.ChevronsUpDown, Disabled: len(p.sources) == 0}
	anyOpen := false
	for i := range p.sources {
		if p.branchOpen(i) {
			anyOpen = true
			break
		}
	}
	if anyOpen {
		fold.Label, fold.Icon = "全部收起", icons.ChevronsDownUp
	}
	if kit.IconButton(f, theme, fold) {
		p.foldOverride = true
		if p.collapsed == nil {
			p.collapsed = make(map[int]bool)
		}
		for i := range p.sources {
			p.collapsed[i] = anyOpen
		}
	}
}

// filter keeps the query in step with the search box. The text is read and
// compared once per change, so a frame that changes nothing costs nothing.
func (p *Page) filter() {
	query := strings.ToLower(strings.TrimSpace(p.search.Text()))
	if query != p.query {
		p.query = query
		p.foldOverride = false
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
	return p.query != "" && !p.foldOverride || !p.collapsed[branch]
}

func (p *Page) toggleBranch(branch int) {
	open := p.branchOpen(branch)
	if p.query != "" && !p.foldOverride {
		clear(p.collapsed)
	}
	p.foldOverride = true
	if p.collapsed == nil {
		p.collapsed = map[int]bool{}
	}
	p.collapsed[branch] = open
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
	p.wakeFolder()
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
		return "仓库须为 owner/repo，@ 为保留分隔符"
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

// drawSourceSwitch aggregates every skill, including ones hidden by search.
func (p *Page) drawSourceSwitch(f ui.C, theme gfx.Theme, branch int) {
	source := p.sources[branch]
	enabled, available := 0, 0
	for _, skill := range source.Skills {
		if skill.Expired {
			continue
		}
		available++
		if skill.Enabled {
			enabled++
		}
	}
	all := available > 0 && enabled == available
	if enabled > 0 && !all {
		ui.Text(f, "部分").FontSize(10).Muted().Shrink(0)
	}
	p.sourceSwitchValues[branch] = all
	if ui.Switch(f, &p.sourceSwitchValues[branch], "").Small().Shrink(0).
		Label(p.branchSwitchLabels[branch]).Disabled(p.updating || p.installing || available == 0).Clicked() {
		p.setEnabled(source.Name, "", p.sourceSwitchValues[branch])
	}
}

func (p *Page) drawSkillSwitch(f ui.C, theme gfx.Theme, branch, leaf int) {
	source := p.sources[branch]
	skill := source.Skills[leaf]
	if ui.Switch(f, &p.sources[branch].Skills[leaf].Enabled, "").Small().Shrink(0).
		Label(p.leafSwitchLabels[branch][leaf]).Disabled(p.updating || p.installing || skill.Expired).Clicked() {
		p.setEnabled(source.Name, skill.Name, p.sources[branch].Skills[leaf].Enabled)
	}
}

func (p *Page) setEnabled(source, skill string, enabled bool) {
	err := p.store.SetEnabled(source, skill, enabled)
	p.updateFailed = err != nil
	switch {
	case err != nil:
		p.updateStatus = "切换失败：" + err.Error()
	case enabled:
		p.updateStatus = "已启用"
	default:
		p.updateStatus = "已关闭"
	}
	p.reloadSources()
}

// Header stays above the independent document and preview scroll areas.
func (p *Page) Header(f ui.C, theme gfx.Theme) {
	p.expiryVisible = false
	ui.Column(f).Height(32).Shrink(0).FillWidth().Children(func() {
		ui.Row(f).Basis(0).Grow(1).FillWidth().PadSides(0, 4, 0, 16).Gap(12).Items(ui.Center).MovesWindow().Children(func() {
			name := p.main.document.Name
			if name == "" {
				name = p.Title()
			}
			ui.Text(f, name).FontSize(12).Bold().Basis(0).Grow(1).Ellipsis()
			if p.chosen.skill == "" {
				return
			}
			if kit.IconButton(f, theme, kit.IconButtonSpec{ID: p.ids.skillFolder, Label: "技能文件夹", Icon: icons.FolderOpen, Disabled: p.openingFolder}) {
				p.openFolder(true)
			}
			if p.main.err == nil {
				ui.Text(f, p.characterLabel).FontSize(11).Muted().Shrink(0)
			}
			// Keep the label with its control; the switch already reserves a generous hit area.
			ui.Row(f).Shrink(0).Gap(0).Items(ui.Center).Children(func() {
				ui.Text(f, "过期").FontSize(11).Muted().Shrink(0)
				width, height := components.SwitchHitExtent(theme, true)
				p.expirySlot = ui.Slot(f).Size(width, height).Shrink(0).MarginSides(0, 0, 0, -4)
				p.expiryVisible = true
			})
		})
		ui.Row(f).Height(1).Shrink(0).FillWidth().Background(theme.Border)
	})
}

// paintExpiry uses Viva's switch with a local danger accent for this state only.
func (p *Page) paintExpiry(canvas *gfx.Canvas, theme gfx.Theme) {
	if !p.expiryVisible {
		return
	}
	expired := false
	for _, source := range p.sources {
		if source.Name != p.chosen.source {
			continue
		}
		for _, skill := range source.Skills {
			if skill.Name == p.chosen.skill {
				expired = skill.Expired
			}
		}
	}
	area := p.expirySlot.Rect()
	width, height := components.SwitchExtent(theme, true)
	bounds := gfx.Rect{X: area.X + (area.Width-width)/2, Y: area.Y + (area.Height-height)/2, Width: width, Height: height}
	theme.Primary, theme.PrimaryForeground = theme.Danger, theme.DangerForeground
	control := components.BeginSwitch(canvas, theme, p.ids.expired, bounds, expired).Small().Label("过期")
	if p.updating || p.installing {
		control = control.Disabled()
	}
	if !control.Paint().Clicked() || p.updating || p.installing {
		return
	}
	err := p.store.SetExpired(p.chosen.source, p.chosen.skill, !expired)
	p.updateFailed = err != nil
	switch {
	case err != nil:
		p.updateStatus = "设置过期失败：" + err.Error()
	case expired:
		p.updateStatus = "已取消过期，技能保持关闭"
	default:
		p.updateStatus = "已标记过期并停用"
	}
	p.reloadSources()
	if p.invalidator != nil {
		p.invalidator.Invalidate()
	}
}

// openFolder lets the file manager launch without blocking the window thread.
func (p *Page) openFolder(skill bool) {
	p.openingFolder = true
	chosen := p.chosen
	go func() {
		var err error
		if skill {
			err = p.store.OpenSkillFolder(chosen.source, chosen.skill)
		} else {
			err = p.store.OpenFolder()
		}
		p.mu.Lock()
		p.folderDone, p.folderErr = true, err
		p.mu.Unlock()
		if p.invalidator != nil {
			p.invalidator.Invalidate()
		}
	}()
}

func (p *Page) wakeFolder() {
	p.mu.Lock()
	done, err := p.folderDone, p.folderErr
	p.folderDone, p.folderErr = false, nil
	p.mu.Unlock()
	if !done {
		return
	}
	p.openingFolder = false
	if err != nil {
		p.updateStatus, p.updateFailed = "打开目录失败："+err.Error(), true
	}
}

// compareSkills keeps actionable skills first and expired skills last.
func compareSkills(a, b domain.Skill) int {
	rank := func(s domain.Skill) int {
		if s.Expired {
			return 2
		}
		if s.Enabled {
			return 0
		}
		return 1
	}
	if delta := rank(a) - rank(b); delta != 0 {
		return delta
	}
	return strings.Compare(a.Name, b.Name)
}
