package shell

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"

	"github.com/lizhian/viva-agents-manager/internal/domain"
	"github.com/lizhian/viva-agents-manager/internal/kit"
)

// newWindow builds the window over the placeholder store, driven without a
// window: the driver runs the real painter and input router.
func newWindow(t *testing.T) (*Shell, *app.Driver) {
	t.Helper()
	window := New(domain.NewMemory())
	var canvas ui.Canvas
	driver, err := app.NewDriver(window.Config(), &canvas)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	return window, driver
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
