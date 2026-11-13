package agentsmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lizhian/viva-agents-manager/internal/instructions"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

func testPage(t testing.TB) (*Page, app.Config) { return testPageTheme(t, gfx.LightTheme()) }
func testPageTheme(t testing.TB, theme gfx.Theme) (*Page, app.Config) {
	t.Helper()
	root := t.TempDir()
	var tree gfx.Tree
	invalidator := &app.Invalidator{}
	p := New(&tree, tree.Feature("instructions"), Options{Invalidator: invalidator, Storage: instructions.Config{Root: filepath.Join(root, "library"), Target: filepath.Join(root, "agents", "AGENTS.md")}})
	p.ensure()
	if p.loadErr != "" {
		t.Fatal(p.loadErr)
	}
	id := p.store.Data.Add("中文回复", "# 回答规范\n\n请使用中文。\n\n```go\nfmt.Println(\"hello\")\n```\n")
	p.store.Data.Snippet(id).Enabled = true
	p.save(true)
	p.selectSnippet(id)
	page := ui.NewPage(ui.PageConfig{Tree: &tree, Theme: &theme})
	config := app.Config{Invalidator: invalidator, Tree: &tree, Width: 1100, Height: 720, MinWidth: 520, MinHeight: 400}
	config.Paint = func(c *gfx.Canvas, size gfx.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		c.Fill(bounds, theme.Background)
		page.Draw(c, bounds, func(f ui.C) {
			ui.Row(f).Fill().Children(func() {
				p.drawPane(f, theme, p.split.Pos)
				ui.Column(f).Grow(1).FillWidth().Children(func() { p.Content(f, theme) })
			})
			p.Dialog(f, theme)
		})
		p.Overlay(c, theme)
	}
	config.TextInput = p.TextInput
	config.OnKey = p.Key
	config.OnShortcut = p.Shortcut
	config.OnPointer = func(e gfx.PointerEvent) gfx.EventResult {
		if r := page.Pointer(e); r.Handled {
			return r
		}
		return p.Pointer(e)
	}
	config.OnScroll = func(e gfx.ScrollEvent) gfx.EventResult {
		if r := page.Wheel(e); r.Handled {
			return r
		}
		return p.Scroll(e)
	}
	return p, config
}
func click(t *testing.T, d *app.Driver, label string) {
	t.Helper()
	for _, h := range d.Snapshot().Hits {
		if h.Label == label {
			if err := d.Click(h.Bounds.X+h.Bounds.Width/2, h.Bounds.Y+h.Bounds.Height/2); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("missing control %q", label)
}
func TestPageEditsGeneratesAndSwitches(t *testing.T) {
	p, config := testPage(t)
	var canvas gfx.Canvas
	d, err := app.NewDriver(config, &canvas)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	click(t, d, "Markdown 源码")
	if err := d.Type("\n新增指令"); err != nil {
		t.Fatal(err)
	}
	if !p.flush() {
		t.Fatal(p.errorText)
	}
	b, _ := os.ReadFile(p.store.Config.Target)
	if string(b) != p.store.Data.Merge() {
		t.Fatal("editor did not regenerate")
	}
	click(t, d, "启用片段 中文回复")
	b, _ = os.ReadFile(p.store.Config.Target)
	if len(b) != 0 {
		t.Fatal("switch did not disable output")
	}
	click(t, d, "预览")
	if p.editor.mode != "preview" {
		t.Fatal("mode not switched")
	}
}
func TestEditorCompositionNotSavedUntilCommit(t *testing.T) {
	p, _ := testPage(t)
	v := p.editor
	d := p.buffer(v.selected)
	input := v.textInput(app.TextInputContext{ID: v.ids["body"]})
	input.OnEvent(app.TextEvent{Kind: app.TextSetMarked, Text: "输入", MarkedSelection: app.TextRange{Start: 2, End: 2}})
	if p.flush() {
		t.Fatal("saved composing text")
	}
	input.OnEvent(app.TextEvent{Kind: app.TextUnmark})
	if !p.flush() {
		t.Fatal("committed text not saved")
	}
	if p.store.Data.Snippet(v.selected).Body != d.body.String() {
		t.Fatal("editor body not persisted")
	}
}

func TestSaveFailureRetryStillGeneratesBody(t *testing.T) {
	p, _ := testPage(t)
	d := p.buffer(p.editor.selected)
	d.body.Insert("changed")
	d.dirty = true
	original := p.store.Config.Root
	p.store.Config.Root = p.store.Config.Target
	if p.flush() {
		t.Fatal("expected failure")
	}
	p.store.Config.Root = original
	if !p.flush() {
		t.Fatal(p.errorText)
	}
	b, _ := os.ReadFile(p.store.Config.Target)
	if string(b) != p.store.Data.Merge() {
		t.Fatal("retry saved source without generating")
	}
}
func TestSearchFiltersFlatSnippets(t *testing.T) {
	p, config := testPage(t)
	id := p.store.Data.Add("match", "A")
	p.store.Data.Add("hidden", "B")
	p.save(false)
	var canvas gfx.Canvas
	d, err := app.NewDriver(config, &canvas)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	click(t, d, "搜索标题、正文和标签")
	d.Type("match")
	for _, hit := range d.Snapshot().Hits {
		if hit.Label == "hidden" {
			t.Fatal("search did not filter")
		}
	}
	click(t, d, "启用片段 match")
	if !p.store.Data.Snippet(id).Enabled {
		t.Fatal("filtered snippet switch did not work")
	}
}

func TestLayoutAndScreenshots(t *testing.T) {
	p, config := testPageTheme(t, gfx.DarkTheme())
	config.Width = 1269
	config.Height = 870
	p.store.Data.Snippet(p.editor.selected).Tags = []string{"编码规范"}
	p.save(false)
	session, err := headless.Open(config, nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if problems := headless.CheckLayout(session.Snapshot()); len(problems) > 0 {
		for _, problem := range problems {
			t.Logf("layout: %+v", problem)
		}
		t.Fail()
	}
	if dir := os.Getenv("VAM_AGENTSMd_SHOTS"); dir != "" {
		os.MkdirAll(dir, 0700)
		if err := session.Screenshot(filepath.Join(dir, "agentsmd-source.png")); err != nil {
			t.Fatal(err)
		}
		p.editor.mode = "split"
		config.Invalidator.Invalidate()
		session.Driver().Pump()
		if err := session.Screenshot(filepath.Join(dir, "agentsmd-split.png")); err != nil {
			t.Fatal(err)
		}
	}
	for _, mode := range []string{"source", "preview", "split"} {
		p.editor.mode = mode
		config.Invalidator.Invalidate()
		session.Driver().Pump()
		if err := session.Driver().Resize(720, 600); err != nil {
			t.Fatal(err)
		}
		for _, problem := range headless.CheckLayout(session.Snapshot()) {
			t.Errorf("narrow %s: %+v", mode, problem)
		}
	}
}
func BenchmarkWorkspacePaint(b *testing.B) {
	_, config := testPage(b)
	var canvas gfx.Canvas
	d, err := app.NewDriver(config, &canvas)
	if err != nil {
		b.Fatal(err)
	}
	defer d.Close()
	config.Invalidator.Invalidate()
	d.Pump()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config.Invalidator.Invalidate()
		if err := d.Pump(); err != nil {
			b.Fatal(err)
		}
	}
}

func TestDragReordersSnippets(t *testing.T) {
	p, config := testPage(t)
	id := p.store.Data.Add("第二个片段", "second")
	p.store.Data.Enable(id, true)
	p.save(true)
	var canvas gfx.Canvas
	d, err := app.NewDriver(config, &canvas)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	bounds := func(label string) gfx.Rect {
		for _, hit := range d.Snapshot().Hits {
			if hit.Label == label {
				return hit.Bounds
			}
		}
		t.Fatalf("missing %s", label)
		return gfx.Rect{}
	}
	from, to := bounds("拖动排序 第二个片段"), bounds("中文回复")
	d.Press(from.X+from.Width/2, from.Y+from.Height/2, gfx.PointerButtonPrimary)
	d.Move(to.X+to.Width/2, to.Y+to.Height/2)
	if err := d.Release(to.X+to.Width/2, to.Y+to.Height/2, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if p.store.Data.Entries()[0].ID != id {
		t.Fatal("drag did not reorder")
	}
	b, _ := os.ReadFile(p.store.Config.Target)
	if !strings.HasPrefix(string(b), "second\n") {
		t.Fatal("drag did not regenerate output")
	}
	restored, err := instructions.Open(p.store.Config)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Data.Entries()[0].ID != id {
		t.Fatal("drag order was not persisted")
	}
	// Dropping the first item into the list's empty space moves it to the end.
	from, to = bounds("拖动排序 第二个片段"), bounds("片段列表")
	x, y := to.X+to.Width/2, to.Y+to.Height-8
	if err := d.Press(from.X+from.Width/2, from.Y+from.Height/2, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := d.Move(x, y); err != nil {
		t.Fatal(err)
	}
	if err := d.Release(x, y, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if p.store.Data.Entries()[1].ID != id {
		t.Fatal("drag to end failed")
	}
	b, _ = os.ReadFile(p.store.Config.Target)
	if !strings.HasSuffix(string(b), "second\n") {
		t.Fatal("drag to end did not regenerate")
	}

}

func TestExternalModificationResolutionInPage(t *testing.T) {
	p, config := testPage(t)
	var canvas gfx.Canvas
	d, err := app.NewDriver(config, &canvas)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := os.WriteFile(p.store.Config.Target, []byte("external instructions"), 0600); err != nil {
		t.Fatal(err)
	}
	p.save(true)
	config.Invalidator.Invalidate()
	d.Pump()
	click(t, d, "处理外部修改")
	for _, problem := range headless.CheckLayout(d.Snapshot()) {
		t.Errorf("resolve layout: %+v", problem)
	}
	click(t, d, "导入为关闭片段后重新生成")
	if p.store.Data.Paused || len(p.store.Data.Snippets) != 2 || p.store.Data.Snippets[1].Enabled {
		t.Fatal("resolution failed")
	}
}

func TestTagsPopoverFiltersTogglesAndAdds(t *testing.T) {
	p, config := testPage(t)
	selected := p.editor.selected
	p.store.Data.Snippet(selected).Tags = []string{"333"}
	other := p.store.Data.Add("标签来源", "")
	p.store.Data.Snippet(other).Tags = []string{"1", "2"}
	p.save(false)
	session, err := headless.Open(config, nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.ClickLabel("管理标签"); err != nil {
		t.Fatal(err)
	}
	if err := session.Driver().Advance(250 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	for _, problem := range headless.CheckLayout(session.Snapshot()) {
		t.Errorf("tag popover: %+v", problem)
	}
	if dir := os.Getenv("VAM_AGENTSMd_SHOTS"); dir != "" {
		if err := session.Screenshot(filepath.Join(dir, "tags-popover.png")); err != nil {
			t.Fatal(err)
		}
	}
	if err := session.Type("1"); err != nil {
		t.Fatal(err)
	}
	if err := session.ClickLabel("1"); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(p.store.Data.Snippet(selected).Tags, "1") || !p.tagsOpen {
		t.Fatal("tag choice should toggle immediately and leave popover open")
	}
	if err := session.ClickLabel("搜索或添加"); err != nil {
		t.Fatal(err)
	}
	session.Driver().Shortcut('a')
	if err := session.Type("new, second"); err != nil {
		t.Fatal(err)
	}
	if err := session.ClickLabel("新增"); err != nil {
		t.Fatal(err)
	}
	for _, tag := range []string{"new", "second"} {
		if !slices.Contains(p.store.Data.Snippet(selected).Tags, tag) {
			t.Fatalf("new tag missing: %s", tag)
		}
	}
	if p.tagQuery.Text() != "" || !p.tagsOpen {
		t.Fatal("adding should clear query without closing popover")
	}
	if err := session.Type("enter-tag"); err != nil {
		t.Fatal(err)
	}
	if err := session.Driver().Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(p.store.Data.Snippet(selected).Tags, "enter-tag") {
		t.Fatal("Enter did not add tag")
	}

	if err := session.ClickLabel("✓ 1"); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(p.store.Data.Snippet(selected).Tags, "1") {
		t.Fatal("tag did not deselect")
	}
	if err := session.Driver().Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if p.tagsOpen {
		t.Fatal("Escape did not close popover")
	}
	restored, err := instructions.Open(p.store.Config)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(restored.Data.Snippet(selected).Tags, "new") {
		t.Fatal("tag edits not persisted")
	}
}
