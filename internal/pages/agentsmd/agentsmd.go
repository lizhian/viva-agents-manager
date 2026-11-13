// Package agentsmd is the local instruction library's editing workspace.
package agentsmd

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/lizhian/viva-agents-manager/internal/instructions"
	"github.com/lizhian/viva-agents-manager/internal/kit"
	"github.com/lizhian/viva-agents-manager/internal/pages"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

type Options struct {
	TitlebarTrailing float32
	Storage          instructions.Config
	Invalidator      *app.Invalidator
}
type draft struct {
	title, body         *text.Editor
	seenTitle, seenBody string
	persistedBody       string
	dirty               bool
	due                 time.Time
}
type row struct {
	instructions.Entry
	id, before, grip              gfx.ID
	label, beforeLabel, dragLabel string
}
type Page struct {
	pages.Base
	tree               *gfx.Tree
	scope              gfx.Scope
	opts               Options
	store              *instructions.Store
	loadErr, errorText string
	saveFailed         bool
	opened             bool
	view               instructions.View
	editor             *editorView
	drafts             map[string]*draft
	ids                map[string]gfx.ID
	fields             app.TextInputProvider
	rows               []row
	visible            map[string]bool
	query              string
	search             ui.Field
	pane               pages.Pane
	split              ui.SplitterState
	modal              *kit.Modal
	action, target     string
	name               ui.Field
	tagQuery           ui.Field
	tagsOpen           bool
	tagEditor          text.Editor
	tagFocus           bool
	tagWords           []string
	tagChoices         []tagChoice
	observed           instructions.Observation
	pendingClose       func()
	upToDate           bool
	tagOptions         []string
	dragState          ui.DragState
	acceptDrop         func(gfx.ID, gfx.ID) bool
	dropOn             func(gfx.ID, gfx.ID)
	imageMu            sync.Mutex
	imageResults       []imageResult
}

func New(tree *gfx.Tree, scope gfx.Scope, opts Options) *Page {
	p := &Page{tree: tree, scope: scope, opts: opts, drafts: map[string]*draft{}, ids: map[string]gfx.ID{}}
	for _, name := range []string{"search", "list", "add", "body", "name", "tags", "tag-query", "root", "resolve", "retry", "preview", "source", "split", "format", "find", "find-prev", "find-next", "find-close", "source-scroll", "preview-scroll", "preview-thumb", "preview-text", "load-images", "links", "copy-code", "title", "splitter"} {
		p.ids[name] = tree.ID(scope, name)
	}
	p.modal = kit.NewModal(tree, scope, "dialog")
	p.split.Pos = 244
	p.pane = pages.Pane{Split: &p.split, Min: 196, Draw: p.drawPane}
	p.editor = newEditor(p, scope)
	p.tagQuery.Bind(&p.tagEditor)
	p.fields = ui.Fields(p.Fields()...)
	p.acceptDrop = p.canDrop
	p.dropOn = p.drop
	return p
}
func (p *Page) Name() string { return "AGENTS.md" }
func (p *Page) Pane() *pages.Pane {
	if p.view.Hidden {
		return nil
	}
	return &p.pane
}
func (p *Page) Fields() []*ui.Field {
	return []*ui.Field{&p.search, &p.name, &p.tagQuery, &p.editor.titleField, &p.editor.findField}
}
func (p *Page) ensure() {
	if p.opened {
		return
	}
	p.opened = true
	config := p.opts.Storage
	if config.Root == "" {
		var err error
		config, err = instructions.DefaultConfig()
		if err != nil {
			p.loadErr = err.Error()
			return
		}
	}
	var err error
	p.store, err = instructions.Open(config)
	if err != nil {
		p.loadErr = err.Error()
		return
	}
	p.view = p.store.LoadView()
	p.split.Pos = p.view.Width
	if err = p.store.Initialize(); err != nil {
		p.result(err)
	}
	p.editor.selected = p.view.Selected
	p.editor.mode = p.view.Mode
	p.editor.split.Pos = p.view.EditorSplit
	if p.store.Data.Snippet(p.editor.selected) == nil {
		if len(p.store.Data.Snippets) > 0 {
			p.editor.selected = p.store.Data.Snippets[0].ID
		} else {
			p.editor.selected = ""
		}
	}
	p.rebuild()
	p.editor.bind()
}
func (p *Page) result(err error) {
	if err != nil {
		p.errorText = err.Error()
	} else {
		p.errorText = ""
	}
	p.invalidate()
}
func (p *Page) invalidate() {
	if p.opts.Invalidator != nil {
		p.opts.Invalidator.Invalidate()
	}
}
func (p *Page) rebuild() {
	if p.store == nil {
		return
	}
	p.rows = p.rows[:0]
	for _, e := range p.store.Data.Entries() {
		scope := p.tree.Sub(p.scope, p.tree.Name(e.ID))
		label := "启用片段 " + e.Name
		p.rows = append(p.rows, row{Entry: e, id: p.tree.ID(scope, "row"), before: p.tree.ID(scope, "before"), grip: p.tree.ID(scope, "grip"), dragLabel: "拖动排序 " + e.Name, label: label, beforeLabel: "移到 " + e.Name + " 之前"})
	}
	p.visible = p.visibleRows()
	p.upToDate = p.store.UpToDate()
	tags := map[string]bool{}
	p.tagOptions = p.tagOptions[:0]
	for _, n := range p.store.Data.Snippets {
		for _, tag := range n.Tags {
			if !tags[tag] {
				tags[tag] = true
				p.tagOptions = append(p.tagOptions, tag)
			}
		}
	}
	slices.Sort(p.tagOptions)
	p.tagChoices = p.tagChoices[:0]
	for _, tag := range p.tagOptions {
		p.tagChoices = append(p.tagChoices, tagChoice{tag: tag, lower: strings.ToLower(tag), selectedLabel: "✓ " + tag})
	}
}
func (p *Page) buffer(id string) *draft {
	if d := p.drafts[id]; d != nil {
		return d
	}
	if p.store == nil {
		return nil
	}
	n := p.store.Data.Snippet(id)
	if n == nil {
		return nil
	}
	d := &draft{title: text.NewEditor(n.Name), body: text.NewEditor(n.Body), seenTitle: n.Name, seenBody: n.Body, persistedBody: n.Body}
	d.body.SetCode(text.CodeOptions{TabWidth: 4, AutoIndent: true, AutoClose: true})
	p.drafts[id] = d
	return d
}
func (p *Page) save(generate bool) bool {
	if p.store == nil || p.loadErr != "" {
		return false
	}
	err := p.store.Save()
	p.saveFailed = err != nil
	if err == nil && generate {
		err = p.store.Generate()
	}
	p.rebuild()
	p.result(err)
	return err == nil || errors.Is(err, instructions.ErrConflict)
}
func (p *Page) flush() bool {
	if p.store == nil {
		return p.loadErr == ""
	}
	generate, dirty := false, false
	for id, d := range p.drafts {
		n := p.store.Data.Snippet(id)
		if n == nil {
			continue
		}
		_, bc := d.body.Composition()
		_, tc := d.title.Composition()
		if bc || tc {
			return false
		}
		title, body := d.title.String(), d.body.String()
		if title != n.Name || body != n.Body || d.dirty {
			dirty = true
			d.dirty = true
			generate = generate || body != d.persistedBody
			n.Name = title
			n.Body = body
			n.Updated = time.Now()
		}
	}
	if !dirty {
		if p.saveFailed {
			return p.save(true)
		}
		return true
	}
	if !p.save(generate) {
		return false
	}
	for _, d := range p.drafts {
		d.dirty = false
		d.seenTitle = d.title.String()
		d.seenBody = d.body.String()
		d.persistedBody = d.seenBody
	}
	return true
}
func (p *Page) autosave(c *gfx.Canvas) {
	if p.store == nil {
		return
	}
	for _, d := range p.drafts {
		title, body := d.title.String(), d.body.String()
		if title != d.seenTitle || body != d.seenBody {
			d.seenTitle = title
			d.seenBody = body
			d.dirty = true
			d.due = time.Now().Add(300 * time.Millisecond)
			p.invalidate()
		}
		_, bc := d.body.Composition()
		_, tc := d.title.Composition()
		if !d.dirty || bc || tc {
			continue
		}
		remaining := time.Until(d.due)
		if remaining > 0 {
			c.RequestFrameIn(remaining)
		} else if !d.due.IsZero() {
			d.due = time.Time{}
			c.AfterPaint(func() { p.flush() })
		}
	}
}
func (p *Page) BeforeLeave() bool {
	if p.flush() {
		p.persistView()
		return true
	}
	p.showClose(nil)
	return false
}
func (p *Page) RequestClose(allow func()) {
	if p.flush() {
		p.persistView()
		allow()
		return
	}
	p.showClose(allow)
}
func (p *Page) showClose(allow func()) {
	p.pendingClose = allow
	p.action = "close"
	p.modal.Open = true
}
func (p *Page) persistView() {
	if p.store == nil || p.loadErr != "" {
		return
	}
	p.view.Selected = p.editor.selected
	p.view.Mode = p.editor.mode
	p.view.Width = p.split.Pos
	p.view.EditorSplit = p.editor.split.Pos
	if err := p.store.SaveView(p.view); err != nil {
		p.result(err)
	}
}
func (p *Page) selectSnippet(id string) {
	if !p.flush() {
		return
	}
	p.tagsOpen = false
	p.editor.selected = id
	p.view.Selected = id
	p.editor.bind()
	p.persistView()
}
func (p *Page) Header(f ui.C, theme gfx.Theme) {}
func (p *Page) Content(f ui.C, theme gfx.Theme) {
	p.ensure()
	if p.loadErr != "" {
		ui.Column(f).Pad(24).Gap(12).Children(func() {
			ui.Text(f, "读取指令库失败").Bold()
			ui.Paragraph(f, p.loadErr, 4).Color(theme.Danger)
			if ui.Button(f, "重新读取").Clicked() {
				p.opened = false
				p.loadErr = ""
			}
		})
		return
	}
	ui.Column(f).Grow(1).FillWidth().Background(theme.Background).PadSides(8, 12, 12, 12).Children(func() {
		if p.store.Data.Paused || p.errorText != "" || !p.upToDate {
			ui.Row(f).FillWidth().Items(ui.Center).Gap(8).Children(func() {
				status := "等待重新生成"
				if p.store.Data.Paused {
					status = "外部修改 · 已暂停生成"
				}
				if p.errorText != "" {
					status = p.errorText
				}
				ui.Text(f, status).FontSize(11).Color(theme.Danger).Ellipsis().Basis(0).Grow(1)
				if p.store.Data.Paused {
					if ui.Button(f, "处理外部修改").XSmall().Clicked() {
						p.resolveDialog()
					}
				} else if ui.Button(f, "重试").XSmall().Clicked() {
					if p.flush() {
						p.result(p.store.Initialize())
						p.rebuild()
					}
				}
			})
			ui.Slot(f).Height(6)
		}
		p.editor.declare(f, theme)
	})
}
func (p *Page) Overlay(c *gfx.Canvas, theme gfx.Theme) {
	if p.loadErr != "" || p.store == nil {
		return
	}
	p.editor.paint(c, theme)
	p.autosave(c)
	if p.tagsOpen && p.tagFocus {
		c.RequestFocus(p.ids["tag-query"])
		p.tagFocus = false
	}
}
func (p *Page) drawPane(f ui.C, theme gfx.Theme, width float32) {
	p.ensure()
	if p.loadErr != "" {
		return
	}
	ui.Column(f).Width(width).Background(theme.Card).PadSides(8, 8, 12, 8).Gap(0).Children(func() {
		ui.Row(f).FillWidth().Height(24).PadSides(0, 0, 0, 0).Gap(6).MovesWindow().Children(func() {
			if ui.TextFieldID(f, p.ids["search"], &p.search, "搜索标题、正文和标签", "搜索片段…").Basis(0).Grow(1).FillWidth().Height(24).FontSize(12).Changed() {
				p.query = p.search.Text()
				p.visible = p.visibleRows()
			}

			if p.icon(f, theme, "add", "新建片段", noteIcons["SquarePen"]) {
				p.create()
			}
		})
		ui.Slot(f).Height(7)
		ui.Divider(f).Background(theme.Border)
		ui.Slot(f).Height(4)
		ui.Scroll(f.Key("instruction-list")).Grow(1).FillWidth().HitWith(p.ids["root"], "片段列表", gfx.HitOptions{Role: gfx.RoleGroup, SkipTab: true}).DropTarget(&p.dragState, p.acceptDrop, p.dropOn).Children(func() {
			visible := p.visible
			for i := range p.rows {
				r := &p.rows[i]
				if !visible[r.ID] {
					continue
				}
				p.drawRow(f, theme, r)
			}
			if len(p.rows) == 0 {
				ui.Paragraph(f, "点击上方新建指令片段。", 3).FontSize(12).Muted().Pad(8)
			}
		})
	})
}
func (p *Page) icon(f ui.C, theme gfx.Theme, id, label string, icon gfx.Icon) bool {
	return iconAction(f, theme, p.ids[id], label, icon)
}
func (p *Page) visibleRows() map[string]bool {
	visible := make(map[string]bool, len(p.rows))
	for _, r := range p.rows {
		visible[r.ID] = instructions.Matches(p.store.Data.Snippet(r.ID), p.query)
	}
	return visible
}
func (p *Page) drawRow(f ui.C, theme gfx.Theme, r *row) {
	strip := ui.Slot(f).FillWidth().Height(2).HitWith(r.before, r.beforeLabel, gfx.HitOptions{Role: gfx.RoleGroup, SkipTab: true}).DropTarget(&p.dragState, p.acceptDrop, p.dropOn)
	if p.dragState.Target() == r.before || p.dragState.Target() == r.id {
		strip.Background(theme.Primary)
	}
	ui.Row(f).FillWidth().Height(30).Items(ui.Center).Gap(2).Children(func() {
		item := ui.Row(f).Basis(0).Grow(1).PadSides(5, 8, 5, 8).Radius(6).Items(ui.Center).Gap(4).Hover(ui.Look{}.Background(theme.Hover)).Hit(r.id, r.Name)
		if p.query == "" {
			item = item.DragSource(&p.dragState).DropTarget(&p.dragState, p.acceptDrop, p.dropOn)
		}
		if p.editor.selected == r.ID {
			item.Background(theme.Secondary)
		}
		if item.Children(func() {
			handle := ui.Row(f).Size(14, 20).Center().HitWith(r.grip, r.dragLabel, gfx.HitOptions{Role: gfx.RoleButton, Cursor: gfx.PointerCursorHand, Disabled: p.query != ""}).Tooltip("拖动调整片段顺序")
			if p.query == "" {
				handle = handle.DragSource(&p.dragState)
			}
			handle.Children(func() { ui.Text(f, "⠿").FontSize(14).Muted() })
			ui.Icon(f, noteIcons["MitMarkdown"]).Size(16, 16)
			ui.Text(f, r.Name).FontSize(12).Color(theme.Foreground).Ellipsis().FillWidth()
		}).Clicked() {
			p.selectSnippet(r.ID)
		}
		before := r.Enabled
		if ui.Switch(f, &r.Enabled, "").Small().Shrink(0).MarginRight(-12).Label(r.label).Clicked() {
			if p.flush() {
				p.store.Data.Enable(r.ID, !before)
				p.save(true)
			}
		}
	})
}
func (p *Page) create() {
	if !p.flush() {
		return
	}
	p.name.SetText("")
	p.action = "create"
	p.modal.Open = true
}
func (p *Page) resolveDialog() {
	o, err := p.store.Observe()
	if err != nil {
		p.result(err)
		return
	}
	p.observed = o
	p.action = "resolve"
	p.modal.Open = true
}
func (p *Page) Dialog(f ui.C, theme gfx.Theme) {
	if !p.modal.Open {
		return
	}
	title := map[string]string{"create": "新建片段", "resolve": "处理外部修改", "close": "草稿尚未保存"}[p.action]
	p.modal.Draw(f, kit.ModalSpec{Title: title, Width: 480}, func() {
		switch p.action {
		case "create":
			ui.TextFieldID(f, p.ids["name"], &p.name, "名称", "名称").FillWidth()
		case "resolve":
			ui.Paragraph(f, "AGENTS.md 已被外部修改。选择如何处理后恢复自动生成。", 3)
		case "close":
			ui.Paragraph(f, "草稿保存失败或仍在组合输入中。可以重试、取消，或放弃未保存修改。", 3)
		}
	}, func() {
		if ui.Button(f.Key("dialog-cancel"), "取消").Outline().Clicked() {
			p.modal.Open = false
			p.pendingClose = nil
		}
		switch p.action {
		case "create":
			if ui.Button(f.Key("dialog-primary"), "保存").Disabled(strings.TrimSpace(p.name.Text()) == "").Clicked() {
				if !p.flush() {
					return
				}
				name := strings.TrimSpace(p.name.Text())
				id := p.store.Data.Add(name, "")
				if p.save(false) {
					p.selectSnippet(id)
				}
				p.modal.Open = false
			}
		case "resolve":
			if ui.Button(f, "导入为关闭片段后重新生成").Clicked() {
				p.resolve(true)
			}
			if ui.Button(f.Key("dialog-primary"), "丢弃修改并重新生成").Clicked() {
				p.resolve(false)
			}
		case "close":
			if ui.Button(f, "放弃修改").Outline().Clicked() {
				p.discard()
				p.modal.Open = false
				if p.pendingClose != nil {
					allow := p.pendingClose
					p.pendingClose = nil
					allow()
				}
			}
			if ui.Button(f.Key("dialog-primary"), "重试保存").Clicked() {
				if p.flush() {
					p.modal.Open = false
					if p.pendingClose != nil {
						allow := p.pendingClose
						p.pendingClose = nil
						allow()
					}
				}
			}
		}
	})
}
func (p *Page) resolve(keep bool) {
	if !p.flush() {
		return
	}
	err := p.store.Resolve(p.observed, keep)
	p.result(err)
	p.rebuild()
	if err == nil {
		p.modal.Open = false
	} else if errors.Is(err, instructions.ErrConflict) {
		p.resolveDialog()
	}
}
func (p *Page) discard() {
	if p.store == nil {
		return
	}
	s, err := instructions.Open(p.store.Config)
	if err != nil {
		p.result(err)
		return
	}
	p.store = s
	p.saveFailed = false
	p.drafts = map[string]*draft{}
	p.editor.bind()
	p.rebuild()
	p.invalidate()
}
func (p *Page) toggleTag(tag string) {
	n := p.store.Data.Snippet(p.target)
	if n == nil {
		return
	}
	for i, t := range n.Tags {
		if t == tag {
			n.Tags = append(n.Tags[:i], n.Tags[i+1:]...)
			p.save(false)
			return
		}
	}
	n.Tags = append(n.Tags, tag)
	p.save(false)
}

// NeedsAttention reports a failed close whose dialog must be brought into view.
func (p *Page) NeedsAttention() bool { return p.modal.Open && p.action == "close" }
