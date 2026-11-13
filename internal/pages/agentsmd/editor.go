package agentsmd

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/markdown"
	"github.com/vivaui/viva/text/find"
	"github.com/vivaui/viva/ui"
)

type editorView struct {
	p                     *Page
	highlights            map[string][]components.CodeEditorRun
	highlightSource       string
	highlight             func(string, string, []components.CodeEditorRun) []components.CodeEditorRun
	cachedTags            []string
	tagLabels             []string
	showTags              bool
	selected, mode        string
	ids                   map[string]gfx.ID
	titleField, findField ui.Field
	input                 app.TextInput
	code                  components.CodeEditor
	doc                   markdown.Document
	rendered              components.MarkdownView
	slot                  ui.Box
	slotted               bool
	bodyWidth             float32
	source, preview       gfx.Rect
	cachedSource          string
	ready                 bool
	width, height         float32
	previewScroll         float32
	previewArea           components.ScrollArea
	previewDrag           bool
	previewGrab           float32
	selection             components.MarkdownSelection
	selecting             bool
	find                  find.Session
	findOpen              bool
	focusFind             bool
	links, copies         gfx.Scope
	formats               gfx.Scope
	modePainter           func(*ui.DrawContext)
	modeOptions           [3]components.ButtonGroupOption
	tagRemoves            gfx.Scope
	splitDragging         bool
	split                 ui.SplitterState
	images                imageState
	updatedLabel          string
	updatedAt             time.Time
}

func newEditor(p *Page, scope gfx.Scope) *editorView {
	v := &editorView{p: p, mode: "source", ids: map[string]gfx.ID{}}
	for _, name := range []string{"title", "body", "find", "find-prev", "find-next", "find-close", "source-scroll", "preview-thumb", "preview-text", "splitter", "tag", "source", "preview", "split", "load-images", "library", "tags", "tag-more"} {
		v.ids[name] = p.tree.ID(scope, name)
	}
	v.links = p.tree.Sub(scope, p.tree.Name("links"))
	v.copies = p.tree.Sub(scope, p.tree.Name("copy-code"))
	v.formats = p.tree.Sub(scope, p.tree.Name("formats"))
	v.tagRemoves = p.tree.Sub(scope, p.tree.Name("tag-removes"))
	v.modePainter = v.drawModes
	v.findField.Bind(&v.find.Query)
	v.highlight = func(_ string, line string, dst []components.CodeEditorRun) []components.CodeEditorRun {
		return append(dst, v.highlights[line]...)
	}
	return v
}
func (v *editorView) bind() {
	v.code = components.CodeEditor{}
	v.ready = false
	v.selection = components.MarkdownSelection{}
	v.images.clear()
	if d := v.p.buffer(v.selected); d != nil {
		v.titleField.Bind(d.title)
	}
}
func (v *editorView) activate() bool { return v.p.buffer(v.selected) != nil }

func (v *editorView) paint(c *gfx.Canvas, theme gfx.Theme) {
	v.source = gfx.Rect{}
	v.preview = gfx.Rect{}
	if !v.slotted {
		return
	}
	d := v.p.buffer(v.selected)
	if d == nil {
		return
	}
	b := v.slot.Rect()
	v.bodyWidth = b.Width
	if b.Width <= 0 || b.Height <= 0 {
		return
	}
	if v.focusFind {
		c.RequestFocus(v.ids["find"])
		v.focusFind = false
	}
	mode := v.mode
	if mode == "split" && b.Width < 640 {
		mode = "source"
	}
	switch mode {
	case "preview":
		v.preview = b
	case "split":
		if v.split.Pos <= 0 || v.split.Pos > b.Width-100 {
			v.split.Pos = b.Width / 2
		}
		v.source = b
		v.source.Width = v.split.Pos - 4
		v.preview = b
		v.preview.X += v.split.Pos + 4
		v.preview.Width -= v.split.Pos + 4
		c.HitWithOptions(v.ids["splitter"], gfx.Rect{X: b.X + v.split.Pos - 3, Y: b.Y, Width: 6, Height: b.Height}, "调整编辑分栏", gfx.HitOptions{Cursor: gfx.PointerCursorResizeHorizontal})
	default:
		v.source = b
	}
	if v.source.Width > 0 {
		if v.highlights == nil || v.highlightSource != d.body.String() {
			v.highlightSource = d.body.String()
			v.highlights = map[string][]components.CodeEditorRun{}
			for _, line := range strings.Split(v.highlightSource, "\n") {
				if _, ok := v.highlights[line]; !ok {
					v.highlights[line] = highlightMarkdown("markdown", line, nil)
				}
			}
		}
		r := d.body.Selection()
		snap := components.CodeEditorSnapshot{ID: v.ids["body"], Label: "Markdown 源码", Value: d.body.String(), Wrap: true, LineNumbers: true, Focused: c.Focused() == v.ids["body"], ShowCaret: true, Caret: d.body.ByteOffset(r.End), SelectionStart: d.body.ByteOffset(r.Start), SelectionEnd: d.body.ByteOffset(r.End), FontSize: 13, TabWidth: 4, Language: "markdown", Highlight: v.highlight, CurrentLine: true, MatchBrackets: true, ScrollbarID: v.ids["source-scroll"]}
		if composition, ok := d.body.Composition(); ok {
			snap.CompositionStart = d.body.ByteOffset(composition.Start)
			snap.CompositionEnd = d.body.ByteOffset(composition.End)
		}
		if v.findOpen {
			v.find.Update(d.body.String())
			snap.Marks = v.find.Matches()
		}
		v.code.Paint(c, theme, v.source, snap)
	}
	if v.preview.Width > 0 {
		v.paintPreview(c, theme, d.body.String())
	}
}
func (v *editorView) paintPreview(c *gfx.Canvas, theme gfx.Theme, source string) {
	b := v.preview
	width := max(1, b.Width-14)
	if !v.ready || v.cachedSource != source || v.width != width || v.images.changed {
		if v.cachedSource != source {
			if v.images.source != source {
				v.images.clear()
			}
			v.selection = components.MarkdownSelection{}
		}
		v.doc.Parse(source)
		v.height = v.rendered.Layout(c, components.MarkdownStyle{Size: 13, LineHeight: 19, CodeInset: 28, Image: v.images.lookup}, &v.doc, width)
		v.cachedSource = source
		v.width = width
		v.ready = true
		v.images.changed = false
	}
	v.previewScroll = min(max(0, v.previewScroll), max(0, v.height-b.Height))
	area := components.BeginScrollArea(c, theme, b, gfx.Size{Width: b.Width, Height: v.height + 12}).Offset(0, v.previewScroll).VerticalScrollbar(v.ids["preview-thumb"], "滚动预览", "")
	v.previewArea = area
	geo := area.BeginContent()
	v.previewScroll = geo.OffsetY
	c.HitWithOptions(v.ids["preview-text"], b, "预览文本", gfx.HitOptions{Role: gfx.RoleGroup, SkipTab: true, ReadOnly: true, Cursor: gfx.PointerCursorText})
	origin := gfx.Point{X: b.X + 6, Y: geo.Content.Y + 6}
	v.rendered.Paint(c, components.MarkdownPaint{Look: components.MarkdownLookFor(theme), X: origin.X, Y: origin.Y, Clip: b, Links: v.links, Selection: v.selection})
	for i, code := range v.rendered.Codes() {
		bounds := code.Bounds
		bounds.X += origin.X
		bounds.Y += origin.Y
		button := gfx.Rect{X: bounds.X + bounds.Width - 30, Y: bounds.Y + 3, Width: 26, Height: 22}
		if contains(b, button.X, button.Y) && contains(b, button.X+button.Width, button.Y+button.Height) {
			components.BeginButton(c, theme, v.copies.At(i), button).Height(22).WithIcon(noteIcons["Copy"]).IconSmall().Ghost("复制代码")
		}
	}
	area.EndContent()
	if index, ok := c.ClickedIn(v.links); ok {
		target := v.rendered.Link(index)
		c.AfterPaint(func() { v.p.result(app.OpenURL(target)) })
	}
	if index, ok := c.ClickedIn(v.copies); ok {
		var codes []string
		for _, block := range v.doc.Blocks {
			if block.Kind == markdown.Code {
				codes = append(codes, block.Text)
			}
		}
		if index < len(codes) {
			value := codes[index]
			c.AfterPaint(func() { v.p.result(app.SetClipboardText(value)) })
		}
	}
}
func contains(r gfx.Rect, x, y float32) bool {
	return x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height
}
func (v *editorView) scroll(e gfx.ScrollEvent) gfx.EventResult {
	if contains(v.source, e.X, e.Y) {
		r := v.code.Scroll(e)
		if v.mode == "split" && v.code.MaxScrollY() > 0 {
			v.previewScroll = v.code.ScrollY() / v.code.MaxScrollY() * max(0, v.height-v.preview.Height)
		}
		return r
	}
	if contains(v.preview, e.X, e.Y) {
		v.previewScroll = max(0, min(max(0, v.height-v.preview.Height), v.previewScroll-e.DeltaY))
		if v.mode == "split" && v.height > v.preview.Height {
			v.code.SetScrollY(v.previewScroll / (v.height - v.preview.Height) * v.code.MaxScrollY())
		}
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	return gfx.EventResult{}
}
func (v *editorView) pointer(e gfx.PointerEvent) gfx.EventResult {
	if e.Kind == gfx.PointerDown && (e.Target == v.ids["body"] || e.Target == v.ids["title"]) {
		if !v.activate() {
			return gfx.EventResult{Handled: true}
		}
	}
	if r := v.code.Pointer(e); r.Handled {
		return r
	}
	if e.Kind == gfx.PointerDown && e.Target == v.ids["splitter"] {
		v.splitDragging = true
	}
	if v.splitDragging {
		if e.Kind == gfx.PointerMove || e.Kind == gfx.PointerUp {
			v.split.Pos = max(100, min(v.slot.Rect().Width-100, e.X-v.slot.Rect().X))
			if e.Kind == gfx.PointerUp {
				v.splitDragging = false
				v.p.persistView()
			}
			return gfx.EventResult{Handled: true, Repaint: true}
		}
	}
	if e.Kind == gfx.PointerDown && e.Target == v.ids["preview-thumb"] {
		v.previewDrag = true
		v.previewGrab = e.Y - v.previewArea.Geometry().VerticalThumb.Y
	}
	if v.previewDrag {
		v.previewScroll = v.previewArea.Geometry().VerticalOffsetForThumb(e.Y - v.previewGrab)
		if e.Kind == gfx.PointerUp {
			v.previewDrag = false
		}
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	point := gfx.Point{X: e.X, Y: e.Y}
	if e.Kind == gfx.PointerDown && contains(v.preview, e.X, e.Y) {
		if offset, ok := v.rendered.OffsetAt(point); ok {
			v.selection = components.MarkdownSelection{Anchor: offset, Focus: offset}
			v.selecting = true
		}
	}
	if v.selecting && (e.Kind == gfx.PointerMove || e.Kind == gfx.PointerUp) {
		if offset, ok := v.rendered.OffsetAt(point); ok {
			v.selection.Focus = offset
		}
		if e.Kind == gfx.PointerUp {
			v.selecting = false
			return gfx.EventResult{Handled: v.selection.Anchor != v.selection.Focus, Repaint: true}
		}
		return gfx.EventResult{Repaint: true}
	}
	return gfx.EventResult{}
}
func (v *editorView) textInput(ctx app.TextInputContext) *app.TextInput {
	if ctx.ID != v.ids["body"] && ctx.ID != v.ids["title"] {
		return nil
	}
	d := v.p.buffer(v.selected)
	if d == nil || v.p.store.Data.Snippet(v.selected) == nil {
		return nil
	}
	editor := d.body
	if ctx.ID == v.ids["title"] {
		editor = d.title
	}
	state := app.TextStateOf(editor)
	v.input = app.TextInput{State: state, NavigationKeys: ctx.NavigationKeys, HideNativeOverlays: ctx.ID == v.ids["body"]}
	id := v.selected
	v.input.OnEvent = func(event app.TextEvent) (app.TextState, error) {
		if v.selected != id || v.p.store.Data.Snippet(id) == nil || v.p.buffer(id) != d {
			return app.TextState{}, nil
		}
		if !v.activate() {
			return app.TextStateOf(editor), nil
		}
		event.Apply(editor)
		if editor == d.body {
			v.code.Reveal(d.body.ByteOffset(d.body.Caret()))
		}
		v.p.invalidate()
		return app.TextStateOf(editor), nil
	}
	return &v.input
}

func (v *editorView) stepFind(forward bool) {
	if !v.activate() {
		return
	}
	d := v.p.buffer(v.selected)
	v.find.Update(d.body.String())
	v.find.Step(d.body, forward)
	v.code.Reveal(d.body.ByteOffset(d.body.Caret()))
}
func (v *editorView) key(e gfx.KeyEvent) gfx.EventResult {
	if e.Release || v.p.modal.Open || v.p.tagsOpen {
		return gfx.EventResult{}
	}
	command := e.Modifiers&(gfx.ModifierMeta|gfx.ModifierControl) != 0
	if command && strings.ToLower(e.Text) == "c" && v.selection.Anchor != v.selection.Focus {
		return gfx.EventResult{Handled: true, ClipboardText: v.rendered.SelectedText(v.selection)}
	}
	if command && strings.ToLower(e.Text) == "f" {
		v.findOpen = true
		v.focusFind = true
		if v.mode == "preview" {
			v.mode = "source"
		}
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	if v.findOpen && e.Target == v.ids["find"] && e.Key == gfx.KeyEnter {
		v.stepFind(e.Modifiers&gfx.ModifierShift == 0)
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	if v.findOpen && e.Key == gfx.KeyEscape {
		v.findOpen = false
		return gfx.EventResult{Handled: true, Repaint: true, Focus: v.ids["body"]}
	}
	if e.Target != v.ids["body"] || !v.activate() {
		return gfx.EventResult{}
	}
	d := v.p.buffer(v.selected)
	if _, composing := d.body.Composition(); composing {
		return gfx.EventResult{}
	}
	if command, ok := components.CodeEditorCommandForKey(e); ok {
		if command.Apply(d.body) {
			v.p.invalidate()
			return gfx.EventResult{Handled: true, Repaint: true}
		}
	}
	if movement, extend := components.TextareaMovementForKey(e.Key, e.Modifiers); movement != components.TextareaMoveNone {
		value := d.body.String()
		offset := v.code.Move(value, d.body.ByteOffset(d.body.Caret()), movement)
		d.body.MoveTo(utf8.RuneCountInString(value[:offset]), extend)
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	return gfx.EventResult{}
}

var formatLabels = []string{"H1", "B", "I", "`", "{ }", "•", "1.", "☑", "❞", "↗", "表格", "—"}

func (v *editorView) format(index int) {
	if !v.activate() {
		return
	}
	d := v.p.buffer(v.selected)
	if _, composing := d.body.Composition(); composing {
		return
	}
	r := d.body.Selection()
	r.Start, r.End = min(r.Start, r.End), max(r.Start, r.End)
	value := []rune(d.body.String())
	selected := string(value[r.Start:r.End])
	prefix, suffix := "", ""
	switch index {
	case 0:
		prefix = "# "
	case 1:
		prefix = "**"
		suffix = "**"
	case 2:
		prefix = "*"
		suffix = "*"
	case 3:
		prefix = "`"
		suffix = "`"
	case 4:
		prefix = "```\n"
		suffix = "\n```"
	case 5:
		prefix = "- "
	case 6:
		prefix = "1. "
	case 7:
		prefix = "- [ ] "
	case 8:
		prefix = "> "
	case 9:
		prefix = "["
		suffix = "](https://)"
	case 10:
		prefix = "\n| 列1 | 列2 |\n| --- | --- |\n| "
		suffix = " |  |\n"
	case 11:
		prefix = "\n\n---\n\n"
	}
	d.body.ReplaceRange(r, prefix+selected+suffix)
	v.p.invalidate()
}

func (v *editorView) openTags() {
	if !v.p.flush() {
		return
	}
	v.p.target = v.selected
	v.p.tagQuery.SetText("")
	v.p.tagWords = nil
	v.p.tagsOpen = !v.p.tagsOpen
}
