package shell

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"

	"github.com/lizhian/viva-agents-manager/internal/domain"
	"github.com/lizhian/viva-agents-manager/internal/kit"
	"github.com/lizhian/viva-agents-manager/internal/library"
)

// newWindow builds the window over the placeholder store, driven without a
// window: the driver runs the real painter and input router.
func newWindow(t *testing.T) (*Shell, *app.Driver) {
	t.Helper()
	return newWindowWith(t, domain.NewMemory())
}

// newWindowWith builds the window over a store of the test's own.
func newWindowWith(t *testing.T, store domain.Store) (*Shell, *app.Driver) {
	t.Helper()
	window := New(store)
	var canvas gfx.Canvas
	driver, err := app.NewDriver(window.Config(), &canvas)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	return window, driver
}

// emptyStore is a store with nothing installed, which is what a first run sees.
type emptyStore struct{ *domain.Memory }

// Sources reports an empty library.
func (emptyStore) Sources() []domain.Source { return nil }

// stubInstaller fetches a repository the way the skills CLI would, without a
// network: two skills, one of which declares itself and is long enough to need
// scrolling.
type stubInstaller struct{}

// stubDeclared is the body of the skill that declares a name and a description.
func stubDeclared() string {
	var body strings.Builder
	body.WriteString("---\nname: Test Skill\ndescription: A skill the tests install.\n---\n\n# Test Skill\n\nSee [DEEPENING.md](DEEPENING.md).\n\n")
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&body, "Paragraph %d of the body.\n\n", i)
	}
	body.WriteString("## The last heading\n")
	return body.String()
}

func (stubInstaller) Prepare(ctx context.Context, repo, dir string) ([]domain.PreparedSkill, error) {
	written := []struct {
		name  string
		files map[string]string
	}{
		{"alpha", map[string]string{
			"SKILL.md":     stubDeclared(),
			"DEEPENING.md": "# Deeper\n\nA file the skill links to.\n",
		}},
		{"long", map[string]string{
			"SKILL.md": "---\nname: Long\ndescription: " + strings.Repeat("A description long enough to want more room than the window has. ", 8) + "\n---\n\n# Long\n",
		}},
		{"tdd", map[string]string{
			"SKILL.md": "# tdd\n\nThe plain body.\n",
		}},
	}
	prepared := make([]domain.PreparedSkill, 0, len(written))
	for _, skill := range written {
		root := filepath.Join(dir, ".agents", "skills", skill.name)
		for name, body := range skill.files {
			if err := os.MkdirAll(root, 0o700); err != nil {
				return nil, err
			}
			if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
				return nil, err
			}
		}
		prepared = append(prepared, domain.PreparedSkill{Name: skill.name, Path: root})
	}
	return prepared, nil
}

// newLibraryWindow builds the window over a real library in a throwaway folder,
// so a test drives the same install path the application does. It returns the
// library's root, for tests that reach into the files.
func newLibraryWindow(t *testing.T) (*Shell, *app.Driver, string) {
	t.Helper()
	dir := t.TempDir()
	store, err := library.New(library.Config{
		Root:       filepath.Join(dir, "skills"),
		EnabledDir: filepath.Join(dir, "enabled"),
		Tmp:        filepath.Join(dir, ".tmp"),
		DBPath:     filepath.Join(dir, "sqlite.db"),
		Installer:  stubInstaller{},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	window, driver := newWindowWith(t, store)
	return window, driver, filepath.Join(dir, "skills")
}

// settle pumps the driver until the window has applied what a background
// install published, the way a window's next loop turn would.
func settle(t *testing.T, driver *app.Driver, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		if done() {
			return
		}
		select {
		case <-driver.Wakes():
		case <-time.After(time.Millisecond):
		}
	}
	t.Fatal("the install did not finish")
}

// drag moves the pointer from one x to another along the splitter row.
func drag(t *testing.T, driver *app.Driver, from, to float32) {
	t.Helper()
	if err := driver.Press(from, 300, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := driver.Move(to, 300); err != nil {
		t.Fatal(err)
	}
	if err := driver.Release(to, 300, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
}

// clickLabel presses the centre of the first hit region with that accessible
// label, which keeps these tests out of the pages' private identities.
func clickLabel(t *testing.T, driver *app.Driver, label string) {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == label {
			centre := hit.Bounds
			if err := driver.Click(centre.X+centre.Width/2, centre.Y+centre.Height/2); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no hit region labelled %q", label)
}

// hitBounds is the rectangle of the first hit region with that label.
func hitBounds(t *testing.T, driver *app.Driver, label string) gfx.Rect {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == label {
			return hit.Bounds
		}
	}
	t.Fatalf("no hit region labelled %q", label)
	return gfx.Rect{}
}

// horizontalBar is the divider that splits a column in two: the one labelled
// "Resize panes" that is a strip rather than a full-height line.
func horizontalBar(t *testing.T, driver *app.Driver) gfx.Rect {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == "Resize panes" && hit.Bounds.Height < 20 {
			return hit.Bounds
		}
	}
	t.Fatal("no horizontal divider is drawn")
	return gfx.Rect{}
}

// drawn reports whether the frame draws that text.
func drawn(t *testing.T, driver *app.Driver, value string) bool {
	t.Helper()
	for _, label := range driver.Snapshot().Labels {
		if label.Value == value {
			return true
		}
	}
	return false
}

// TestSplitterDrag drives the divider the way a pointer would: dragging moves the
// first pane, and the pane stops at the width its labels need.
func TestSplitterDrag(t *testing.T) {
	window, driver := newWindow(t)

	start := window.split.Pos
	drag(t, driver, start+4, start+104)
	if want := start + 100; window.split.Pos != want {
		t.Fatalf("splitter pos after drag = %v, want %v", window.split.Pos, want)
	}

	drag(t, driver, window.split.Pos+4, 0)
	if want := window.natural; window.split.Pos != want {
		t.Fatalf("splitter pos after dragging past the minimum = %v, want %v", window.split.Pos, want)
	}
}

// TestPageSplitterDrag drives the Skills page's own divider, which sits after the
// navigation column and the page column.
func TestPageSplitterDrag(t *testing.T) {
	window, driver := newWindow(t)
	pane := window.page().Pane()
	if pane == nil {
		t.Fatal("the first page has no column")
	}

	start := pane.Split.Pos
	from := window.split.Pos + kit.SplitterWidth + start + 4
	drag(t, driver, from, from+60)
	if want := start + 60; pane.Split.Pos != want {
		t.Fatalf("page splitter pos after drag = %v, want %v", pane.Split.Pos, want)
	}
}

// TestSearchFilters types into the search box and checks that the tree narrows to
// the matching skill.
func TestSearchFilters(t *testing.T) {
	_, driver := newWindow(t)

	clickLabel(t, driver, "搜索 Skills")
	if err := driver.Type("tdd"); err != nil {
		t.Fatal(err)
	}
	if !drawn(t, driver, "tdd") {
		t.Fatal("tdd is not drawn after searching for it")
	}
	for _, gone := range []string{"code-review", "research", "diagnosing-bugs"} {
		if drawn(t, driver, gone) {
			t.Fatalf("%q is still drawn after searching for tdd", gone)
		}
	}
}

// TestBranchCollapse clicks a branch and checks that its leaves stop being drawn,
// then clicks it again to bring them back.
func TestBranchCollapse(t *testing.T) {
	_, driver := newWindow(t)

	branch := "开发"
	if !drawn(t, driver, "tdd") {
		t.Fatal("tdd is not drawn before the branch is folded")
	}
	clickLabel(t, driver, branch)
	if drawn(t, driver, "tdd") {
		t.Fatal("tdd is still drawn after the branch is folded")
	}
	clickLabel(t, driver, branch)
	if !drawn(t, driver, "tdd") {
		t.Fatal("tdd is not drawn after the branch opens again")
	}
}

// TestThemeToggle clicks the sidebar's theme button: the appearance switches and
// the frame is painted in the other theme. The application starts light.
func TestThemeToggle(t *testing.T) {
	window, driver := newWindow(t)

	if window.theme.Dark() {
		t.Fatal("the application did not start in the light theme")
	}
	clickLabel(t, driver, window.nav.Theme.Label())
	if !window.theme.Dark() {
		t.Fatal("the click did not switch to the dark theme")
	}
	if got, want := driver.Snapshot().Primitives[0].Color, gfx.DarkTheme().Background; got != want {
		t.Fatalf("background after the click = %v, want %v", got, want)
	}
	clickLabel(t, driver, window.nav.Theme.Label())
	if window.theme.Dark() {
		t.Fatal("the second click did not return to the light theme")
	}
	if got, want := driver.Snapshot().Primitives[0].Color, gfx.LightTheme().Background; got != want {
		t.Fatalf("background after the second click = %v, want %v", got, want)
	}
}

// TestInstallDialog clicks the toolbar's install button: the dialog opens with its
// source field, cancel closes it, and so does a press on the scrim.
func TestInstallDialog(t *testing.T) {
	_, driver := newWindow(t)

	clickLabel(t, driver, "安装")
	for _, want := range []string{"安装 Skill 来源", "owner/repo", "开始安装", "取消"} {
		if !drawn(t, driver, want) {
			t.Fatalf("%q is not drawn in the dialog", want)
		}
	}
	clickLabel(t, driver, "取消")
	if drawn(t, driver, "安装 Skill 来源") {
		t.Fatal("the dialog is still drawn after cancel")
	}

	// A press on the scrim, far from the surface, dismisses it too.
	clickLabel(t, driver, "安装")
	if !drawn(t, driver, "安装 Skill 来源") {
		t.Fatal("the dialog did not reopen")
	}
	if err := driver.Click(40, 40); err != nil {
		t.Fatal(err)
	}
	if drawn(t, driver, "安装 Skill 来源") {
		t.Fatal("a press on the scrim did not close the dialog")
	}
}

// TestInstallFromTheDialog types a repository and installs it: the install runs
// off the window thread, the dialog closes when its result is applied, and the
// source and its skill are in the tree.
func TestInstallFromTheDialog(t *testing.T) {
	_, driver, _ := newLibraryWindow(t)

	clickLabel(t, driver, "安装")
	clickLabel(t, driver, "GitHub 仓库")
	if err := driver.Type("anthropics/skills"); err != nil {
		t.Fatal(err)
	}
	clickLabel(t, driver, "开始安装")
	settle(t, driver, func() bool { return !drawn(t, driver, "安装 Skill 来源") })

	for _, want := range []string{"anthropics/skills", "tdd"} {
		if !drawn(t, driver, want) {
			t.Fatalf("%q is not in the tree after the install", want)
		}
	}
}

// TestInstallReportsAConflict installs the same repository twice: the second
// attempt keeps the dialog open and says why.
func TestInstallReportsAConflict(t *testing.T) {
	_, driver, _ := newLibraryWindow(t)

	for attempt := 0; attempt < 2; attempt++ {
		clickLabel(t, driver, "安装")
		clickLabel(t, driver, "GitHub 仓库")
		if err := driver.Type("a/b"); err != nil {
			t.Fatal(err)
		}
		clickLabel(t, driver, "开始安装")
		if attempt == 0 {
			settle(t, driver, func() bool { return !drawn(t, driver, "安装 Skill 来源") })
			continue
		}
		settle(t, driver, func() bool { return drawn(t, driver, "这个仓库已经装过了") })
		if !drawn(t, driver, "安装 Skill 来源") {
			t.Fatal("the dialog closed on a failed install")
		}
	}
}

// TestEmptyLibrarySaysSo checks that a first run says the library is empty,
// rather than reporting a search that matched nothing.
func TestEmptyLibrarySaysSo(t *testing.T) {
	_, driver := newWindowWith(t, emptyStore{domain.NewMemory()})
	if !drawn(t, driver, "还没有安装任何技能来源") {
		t.Fatal("an empty library does not say so")
	}
}

// TestPaneButtonsSitAboveTheSearchBox checks the column's header: the three
// buttons share one row over the search box, and they stay inside the column at
// the width the splitter allows, since it can be dragged that narrow.
func TestPaneButtonsSitAboveTheSearchBox(t *testing.T) {
	window, driver := newWindow(t)
	pane := window.page().Pane()
	pane.Split.Pos = pane.Min
	if err := driver.Resize(1100, 720); err != nil {
		t.Fatal(err)
	}

	search := hitBounds(t, driver, "搜索 Skills")
	right := window.split.Pos + kit.SplitterWidth + pane.Split.Pos
	for _, label := range []string{"安装", "更新", "本地文件夹", "全部收起"} {
		hit := hitBounds(t, driver, label)
		if hit.Y+hit.Height > search.Y {
			t.Fatalf("%q is not above the search box", label)
		}
		if hit.X+hit.Width > right {
			t.Fatalf("%q ends at %v, past the column's edge at %v", label, hit.X+hit.Width, right)
		}
	}
}

// installStub drives the dialog to install the stub repository, and waits for
// the window to apply the result.
func installStub(t *testing.T, driver *app.Driver) {
	t.Helper()
	clickLabel(t, driver, "安装")
	clickLabel(t, driver, "GitHub 仓库")
	if err := driver.Type("owner/repo"); err != nil {
		t.Fatal(err)
	}
	clickLabel(t, driver, "开始安装")
	settle(t, driver, func() bool { return !drawn(t, driver, "安装 Skill 来源") })
}

// TestSkillDetailShowsTheDocument checks the detail column: what the document
// declares about itself, and its body.
func TestSkillDetailShowsTheDocument(t *testing.T) {
	_, driver, _ := newLibraryWindow(t)
	installStub(t, driver)

	for _, want := range []string{"Test Skill", "A skill the tests install.", "Paragraph 0 of the body."} {
		if !drawn(t, driver, want) {
			t.Fatalf("%q is not drawn in the detail column", want)
		}
	}
}

// TestSkillDetailWithoutADeclaration checks that a document that declares
// nothing falls back to its directory name, and shows no subtitle.
func TestSkillDetailWithoutADeclaration(t *testing.T) {
	_, driver, _ := newLibraryWindow(t)
	installStub(t, driver)

	clickLabel(t, driver, "tdd")
	if drawn(t, driver, "A skill the tests install.") {
		t.Fatal("another skill's description is still drawn")
	}
	if drawn(t, driver, "SKILL") {
		t.Fatal("a document that declares no name fell back to its file name")
	}
	if !drawn(t, driver, "The plain body.") {
		t.Fatal("the chosen skill's body is not drawn")
	}
}

// TestSkillDetailScrolls checks that a body taller than the column moves with
// the wheel.
func TestSkillDetailScrolls(t *testing.T) {
	_, driver, _ := newLibraryWindow(t)
	installStub(t, driver)

	if drawn(t, driver, "The last heading") {
		t.Fatal("the end of the body is on screen before scrolling")
	}
	if err := driver.Scroll(gfx.ScrollEvent{X: 800, Y: 400, DeltaY: 4000}); err != nil {
		t.Fatal(err)
	}
	if !drawn(t, driver, "The last heading") {
		t.Fatal("the wheel did not move the body")
	}
}

// TestSkillDetailReportsAReadFailure checks that a document that disappeared is
// reported where the body would have been.
func TestSkillDetailReportsAReadFailure(t *testing.T) {
	_, driver, root := newLibraryWindow(t)
	installStub(t, driver)

	if err := os.Remove(filepath.Join(root, "owner@repo", "alpha", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	clickLabel(t, driver, "tdd")
	clickLabel(t, driver, "alpha")
	if !drawn(t, driver, "读取技能文档失败") {
		t.Fatal("a document that cannot be read is not reported")
	}
}

// TestALongDescriptionKeepsTheColumns checks that a skill whose description is
// wider than the window cannot squeeze the navigation or the page's column: a
// flex column's preferred width is the sum of its children's, so the columns
// that must keep their width have to be filled rather than grown.
func TestALongDescriptionKeepsTheColumns(t *testing.T) {
	_, driver, _ := newLibraryWindow(t)
	installStub(t, driver)

	clickLabel(t, driver, "long")
	if nav := hitBounds(t, driver, "Skills"); nav.Width < 80 {
		t.Fatalf("the navigation is %v wide, so a long description squeezed it", nav.Width)
	}
	if search := hitBounds(t, driver, "搜索 Skills"); search.Width < 150 {
		t.Fatalf("the page's column is %v wide, so a long description squeezed it", search.Width)
	}
}

// TestSkillDetailOpensALinkedDocument checks that a link to another file of the
// skill opens it under the skill's own document, and that closing puts the
// document back.
func TestSkillDetailOpensALinkedDocument(t *testing.T) {
	_, driver, _ := newLibraryWindow(t)
	installStub(t, driver)

	if drawn(t, driver, "A file the skill links to.") {
		t.Fatal("the linked document is on screen before it was opened")
	}
	clickLabel(t, driver, "DEEPENING.md")
	if !drawn(t, driver, "A file the skill links to.") {
		t.Fatal("the linked document did not open")
	}
	clickLabel(t, driver, "关闭")
	if drawn(t, driver, "A file the skill links to.") {
		t.Fatal("the preview is still drawn after closing it")
	}
	if !drawn(t, driver, "Paragraph 0 of the body.") {
		t.Fatal("the skill's own document is not back")
	}
}

// TestPreviewSplitterDrag drags the divider between the skill's own document and
// the preview, and checks the preview follows it.
func TestPreviewSplitterDrag(t *testing.T) {
	_, driver, _ := newLibraryWindow(t)
	installStub(t, driver)
	clickLabel(t, driver, "DEEPENING.md")

	header := hitBounds(t, driver, "关闭")
	bar := horizontalBar(t, driver)
	x, y := bar.X+bar.Width/2, bar.Y+bar.Height/2
	if err := driver.Press(x, y, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := driver.Move(x, y-60); err != nil {
		t.Fatal(err)
	}
	if err := driver.Release(x, y-60, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}

	if after := hitBounds(t, driver, "关闭").Y; after >= header.Y {
		t.Fatalf("the preview did not follow the divider: %v then %v", header.Y, after)
	}
}

// slowInstaller waits for the context instead of answering, so a test can watch
// an update that is still running.
type slowInstaller struct{}

func (slowInstaller) Prepare(ctx context.Context, repo, dir string) ([]domain.PreparedSkill, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// newSeededWindow builds the window over a library that already holds one source,
// written by hand, so a test needs no install to get something to update.
func newSeededWindow(t *testing.T, installer domain.Installer) (*Shell, *app.Driver) {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, "skills")
	skill := filepath.Join(root, "owner@repo", "alpha")
	if err := os.MkdirAll(skill, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("# alpha\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := library.New(library.Config{
		Root:      root,
		Tmp:       filepath.Join(dir, ".tmp"),
		DBPath:    filepath.Join(dir, "sqlite.db"),
		Installer: installer,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return newWindowWith(t, store)
}

// TestUpdateReportsItsResult clicks update and checks the status line says what
// happened.
func TestUpdateReportsItsResult(t *testing.T) {
	_, driver, _ := newLibraryWindow(t)
	installStub(t, driver)

	clickLabel(t, driver, "更新")
	settle(t, driver, func() bool { return drawn(t, driver, "1 个来源已更新") })
}

// TestUpdateCanBeCancelled checks that an update in flight offers to stop, and
// that stopping says so.
func TestUpdateCanBeCancelled(t *testing.T) {
	window, driver := newSeededWindow(t, slowInstaller{})
	// Give the toolbar enough room to show the full progress and result text.
	pane := window.page().Pane()
	from := window.split.Pos + kit.SplitterWidth + pane.Split.Pos + 4
	drag(t, driver, from, from+200)

	clickLabel(t, driver, "更新")
	// Wait for the source to be in flight: stopping before it starts is a
	// different path, and one this test is not about.
	settle(t, driver, func() bool { return drawn(t, driver, "正在更新 1/1 · owner/repo") })
	// The button carries no text, so what it offers while a run is in flight is
	// its name: the glyph alone would leave the reader guessing.
	hitBounds(t, driver, "取消")
	clickLabel(t, driver, "取消")
	settle(t, driver, func() bool { return drawn(t, driver, "已取消，0 个来源已更新") })
}

// TestTheHeaderScrollsWithTheBody checks that the skill's title and description
// are part of the document rather than standing still above it.
func TestTheHeaderScrollsWithTheBody(t *testing.T) {
	_, driver, _ := newLibraryWindow(t)
	installStub(t, driver)

	if !drawn(t, driver, "A skill the tests install.") {
		t.Fatal("the description is not drawn")
	}
	if err := driver.Scroll(gfx.ScrollEvent{X: 800, Y: 400, DeltaY: 4000}); err != nil {
		t.Fatal(err)
	}
	if drawn(t, driver, "A skill the tests install.") {
		t.Fatal("the description is still on screen at the end of the document")
	}
}

// TestSelectingTextCopiesIt drags across the document and copies what it chose.
func TestSelectingTextCopiesIt(t *testing.T) {
	_, driver, _ := newLibraryWindow(t)
	installStub(t, driver)

	if err := driver.Press(400, 80, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := driver.Move(700, 220); err != nil {
		t.Fatal(err)
	}
	if err := driver.Release(700, 220, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}

	handled, err := driver.Shortcut('c')
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("the copy shortcut was not handled")
	}
	if text := driver.Clipboard(); !strings.Contains(text, "Test Skill") {
		t.Fatalf("clipboard = %q, want the text the drag chose", text)
	}
}

// TestDraggingOverALinkDoesNotOpenIt checks that a drag is not a click: the link
// it crosses must not open the preview.
func TestDraggingOverALinkDoesNotOpenIt(t *testing.T) {
	_, driver, _ := newLibraryWindow(t)
	installStub(t, driver)

	link := hitBounds(t, driver, "DEEPENING.md")
	y := link.Y + link.Height/2
	if err := driver.Press(link.X+2, y, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := driver.Move(link.X+link.Width-2, y); err != nil {
		t.Fatal(err)
	}
	if err := driver.Release(link.X+link.Width-2, y, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if drawn(t, driver, "A file the skill links to.") {
		t.Fatal("a drag opened the link it crossed")
	}
}

// TestSkillSwitchesManageLinksWithoutSelectingOrFolding drives the independent
// row controls against the real library, including a partially enabled source.
func TestSkillSwitchesManageLinksWithoutSelectingOrFolding(t *testing.T) {
	window, driver, root := newLibraryWindow(t)
	installStub(t, driver)
	title := window.page().(interface{ Title() string }).Title()
	enabledDir := filepath.Join(filepath.Dir(root), "enabled")
	link := func(skill string) string { return filepath.Join(enabledDir, "vam@owner@repo@"+skill) }
	clickLabel(t, driver, "启用技能 owner/repo/tdd")
	if _, err := os.Stat(filepath.Join(link("tdd"), "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if !drawn(t, driver, "部分") {
		t.Fatal("source does not show partial state")
	}
	if window.page().(interface{ Title() string }).Title() != title {
		t.Fatal("switch selected a different skill")
	}
	hitBounds(t, driver, "启用技能 owner/repo/alpha")
	clickLabel(t, driver, "启用来源 owner/repo")
	for _, skill := range []string{"alpha", "long", "tdd"} {
		if _, err := os.Readlink(link(skill)); err != nil {
			t.Fatal(err)
		}
	}
	if drawn(t, driver, "部分") {
		t.Fatal("fully enabled source still shows partial state")
	}
	hitBounds(t, driver, "启用技能 owner/repo/alpha")
	clickLabel(t, driver, "启用来源 owner/repo")
	for _, skill := range []string{"alpha", "long", "tdd"} {
		if _, err := os.Lstat(link(skill)); !os.IsNotExist(err) {
			t.Fatalf("link remains: %s, %v", skill, err)
		}
	}
	if window.page().(interface{ Title() string }).Title() != title {
		t.Fatal("source switch changed selection")
	}
}

// BenchmarkSkillsPaint measures a complete steady-state frame with the tree's
// switches and the selected document, without opening a native window.
func BenchmarkSkillsPaint(b *testing.B) {
	window := New(domain.NewMemory())
	var canvas gfx.Canvas
	driver, err := app.NewDriver(window.Config(), &canvas)
	if err != nil {
		b.Fatal(err)
	}
	defer driver.Close()
	for i := 0; i < 5; i++ {
		window.invalidator.Invalidate()
		if err := driver.Pump(); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		window.invalidator.Invalidate()
		if err := driver.Pump(); err != nil {
			b.Fatal(err)
		}
	}
}

// overflowingSkills keeps the fixture independent of network and user files.
type overflowingSkills struct{ *domain.Memory }

func (overflowingSkills) Sources() []domain.Source {
	skills := make([]domain.Skill, 60)
	for i := range skills {
		skills[i].Name = fmt.Sprintf("skill-%02d", i)
	}
	return []domain.Source{{Name: "many", Skills: skills}}
}

func TestSkillsListScrollsWithoutMovingSearch(t *testing.T) {
	window, driver := newWindowWith(t, overflowingSkills{domain.NewMemory()})
	search := hitBounds(t, driver, "搜索 Skills")
	first := hitBounds(t, driver, "skill-00")
	x := window.split.Pos + kit.SplitterWidth + 50
	if err := driver.Scroll(gfx.ScrollEvent{X: x, Y: first.Y + 10, DeltaY: 10000, Precise: true}); err != nil {
		t.Fatal(err)
	}
	last := hitBounds(t, driver, "skill-59")
	if last.Y+last.Height > driver.Frame().Height || last.Y < search.Y+search.Height {
		t.Fatal("scrolling the skills list did not reveal its last skill")
	}
	if after := hitBounds(t, driver, "搜索 Skills"); after != search {
		t.Fatal("list scrolling moved the search field")
	}
	if err := driver.Scroll(gfx.ScrollEvent{X: x, Y: first.Y + 10, DeltaY: -10000, Precise: true}); err != nil {
		t.Fatal(err)
	}
	hitBounds(t, driver, "skill-00")
}

func TestSkillSwitchStaysRightAlignedWhenPaneWidens(t *testing.T) {
	window, driver := newWindow(t)
	before := hitBounds(t, driver, "启用技能 开发/tdd")
	pane := window.page().Pane()
	from := window.split.Pos + kit.SplitterWidth + pane.Split.Pos + 4
	drag(t, driver, from, from+180)
	after := hitBounds(t, driver, "启用技能 开发/tdd")
	if after.X-before.X != 180 {
		t.Fatalf("switch did not follow the pane right edge: %v to %v", before.X, after.X)
	}
}

func TestDetailToolbarExpirationDisablesListSwitch(t *testing.T) {
	window, driver, root := newLibraryWindow(t)
	installStub(t, driver)
	label := fmt.Sprintf("总字数 %d", domain.ParseDocument([]byte(stubDeclared()), "").CharacterCount)
	if !drawn(t, driver, label) {
		t.Fatal("missing full-file character count")
	}
	header := hitBounds(t, driver, "过期")
	if header.X+header.Width > driver.Frame().Width-window.trailing {
		t.Fatal("expiration switch overlaps native window controls")
	}
	clickLabel(t, driver, "启用技能 owner/repo/alpha")
	clickLabel(t, driver, "过期")
	link := filepath.Join(filepath.Dir(root), "enabled", "vam@owner@repo@alpha")
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatal("expired skill remains linked")
	}
	// A disabled row switch is still announced and visible, but cannot enable it.
	clickLabel(t, driver, "启用技能 owner/repo/alpha")
	clickLabel(t, driver, "启用来源 owner/repo")
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatal("individual or source switch enabled expired skill")
	}
	if err := driver.Scroll(gfx.ScrollEvent{X: 800, Y: 400, DeltaY: 4000}); err != nil {
		t.Fatal(err)
	}
	if after := hitBounds(t, driver, "过期"); after != header {
		t.Fatal("toolbar scrolled with document")
	}
	if !drawn(t, driver, label) || !drawn(t, driver, "Test Skill") {
		t.Fatal("toolbar lost name or count after scrolling")
	}
	clickLabel(t, driver, "过期")
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatal("un-expiring automatically enabled skill")
	}
	clickLabel(t, driver, "启用技能 owner/repo/alpha")
	if _, err := os.Readlink(link); err != nil {
		t.Fatal("switch did not become usable:", err)
	}
}

func TestFolderButtonRevealsEnabledSkills(t *testing.T) {
	_, driver, root := newLibraryWindow(t)
	clickLabel(t, driver, "本地文件夹")
	settle(t, driver, func() bool {
		path, ok := driver.LastRevealedPath()
		return ok && path == filepath.Join(filepath.Dir(root), "enabled")
	})
}

func TestToolbarFoldsAndExpandsAllSources(t *testing.T) {
	_, driver := newWindow(t)
	hasHit := func(name string) bool {
		for _, hit := range driver.Snapshot().Hits {
			if hit.Label == name {
				return true
			}
		}
		return false
	}
	clickLabel(t, driver, "全部收起")
	if hasHit("tdd") || hasHit("research") {
		t.Fatal("collapse all left skills visible")
	}
	clickLabel(t, driver, "全部展开")
	if !hasHit("tdd") || !hasHit("research") {
		t.Fatal("expand all did not reveal skills")
	}
	clickLabel(t, driver, "搜索 Skills")
	if err := driver.Type("tdd"); err != nil {
		t.Fatal(err)
	}
	clickLabel(t, driver, "全部收起")
	if hasHit("tdd") {
		t.Fatal("collapse all did not work during search")
	}
	clickLabel(t, driver, "全部展开")
	if !hasHit("tdd") || hasHit("research") {
		t.Fatal("expand all lost search filter")
	}
}

func TestDetailFolderRevealsInstalledSkill(t *testing.T) {
	_, driver, root := newLibraryWindow(t)
	installStub(t, driver)
	clickLabel(t, driver, "alpha")
	want, err := filepath.EvalSymlinks(filepath.Join(root, "owner@repo", "alpha"))
	if err != nil {
		t.Fatal(err)
	}
	clickLabel(t, driver, "技能文件夹")
	settle(t, driver, func() bool {
		path, ok := driver.LastRevealedPath()
		return ok && path == want
	})
}
