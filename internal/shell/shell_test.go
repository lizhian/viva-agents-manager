package shell

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"

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
	var canvas ui.Canvas
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
// network: one skill in a folder of its own.
type stubInstaller struct{}

func (stubInstaller) Prepare(ctx context.Context, repo, dir string) ([]domain.PreparedSkill, error) {
	root := filepath.Join(dir, ".agents", "skills", "tdd")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("# tdd"), 0o600); err != nil {
		return nil, err
	}
	return []domain.PreparedSkill{{Name: "tdd", Path: root}}, nil
}

// newLibraryWindow builds the window over a real library in a throwaway folder,
// so a test drives the same install path the application does.
func newLibraryWindow(t *testing.T) (*Shell, *app.Driver) {
	t.Helper()
	dir := t.TempDir()
	store, err := library.New(library.Config{
		Root:      filepath.Join(dir, "skills"),
		Tmp:       filepath.Join(dir, ".tmp"),
		DBPath:    filepath.Join(dir, "sqlite.db"),
		Installer: stubInstaller{},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return newWindowWith(t, store)
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
	if err := driver.Press(from, 300, ui.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := driver.Move(to, 300); err != nil {
		t.Fatal(err)
	}
	if err := driver.Release(to, 300, ui.PointerButtonPrimary); err != nil {
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
func hitBounds(t *testing.T, driver *app.Driver, label string) ui.Rect {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == label {
			return hit.Bounds
		}
	}
	t.Fatalf("no hit region labelled %q", label)
	return ui.Rect{}
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
	if got, want := driver.Snapshot().Primitives[0].Color, ui.DarkTheme().Background; got != want {
		t.Fatalf("background after the click = %v, want %v", got, want)
	}
	clickLabel(t, driver, window.nav.Theme.Label())
	if window.theme.Dark() {
		t.Fatal("the second click did not return to the light theme")
	}
	if got, want := driver.Snapshot().Primitives[0].Color, ui.LightTheme().Background; got != want {
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
	_, driver := newLibraryWindow(t)

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
	_, driver := newLibraryWindow(t)

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
	for _, label := range []string{"安装", "更新", "本地文件夹"} {
		hit := hitBounds(t, driver, label)
		if hit.Y+hit.Height > search.Y {
			t.Fatalf("%q is not above the search box", label)
		}
		if hit.X+hit.Width > right {
			t.Fatalf("%q ends at %v, past the column's edge at %v", label, hit.X+hit.Width, right)
		}
	}
}
