package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lizhian/viva-agents-manager/internal/domain"
)

// blockingInstaller waits for the context instead of answering, so a test can
// see an update that is still running.
type blockingInstaller struct {
	// started, when set, is closed once the installer is waiting.
	started chan struct{}
}

func (b blockingInstaller) Prepare(ctx context.Context, repo, dir string) ([]domain.PreparedSkill, error) {
	if b.started != nil {
		close(b.started)
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

// TestUpdateBringsTheSourceInLineWithTheRemote checks the promise: what the
// library holds afterwards is what the repository holds, so a skill the
// repository dropped is gone.
func TestUpdateBringsTheSourceInLineWithTheRemote(t *testing.T) {
	library := newLibrary(t, twoSkills())
	if err := library.Install(context.Background(), "a/b", nil); err != nil {
		t.Fatal(err)
	}
	// The repository changed: one skill left, and the one that stayed says
	// something else.
	library.config.Installer = fakeInstaller{skills: []fakeSkill{
		{Name: "tdd", Files: map[string]string{"SKILL.md": "# tdd, rewritten"}},
	}}

	results := library.UpdateAll(context.Background(), nil)
	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("results = %+v, want one success", results)
	}

	source := filepath.Join(library.config.Root, "a_@_b")
	if _, err := os.Stat(filepath.Join(source, "code-review")); !os.IsNotExist(err) {
		t.Fatal("a skill the repository dropped is still in the library")
	}
	data, err := os.ReadFile(filepath.Join(source, "tdd", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# tdd, rewritten" {
		t.Fatalf("SKILL.md = %q, want what the repository has now", data)
	}
	if sources := library.Sources(); len(sources) != 1 || len(sources[0].Skills) != 1 {
		t.Fatalf("sources = %+v, want one source holding one skill", sources)
	}
}

// TestUpdateKeepsASourceThatFailed checks that a repository that cannot be
// fetched leaves the library alone.
func TestUpdateKeepsASourceThatFailed(t *testing.T) {
	library := newLibrary(t, twoSkills())
	if err := library.Install(context.Background(), "a/b", nil); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("no network")
	library.config.Installer = fakeInstaller{err: failure}

	results := library.UpdateAll(context.Background(), nil)
	if len(results) != 1 || !errors.Is(results[0].Err, failure) {
		t.Fatalf("results = %+v, want the failure", results)
	}
	if sources := library.Sources(); len(sources) != 1 || len(sources[0].Skills) != 2 {
		t.Fatalf("sources = %+v, want the source untouched", sources)
	}
}

// TestUpdateCarriesOnPastAFailure checks that one broken repository does not
// stop the others.
func TestUpdateCarriesOnPastAFailure(t *testing.T) {
	library := newLibrary(t, twoSkills())
	for _, repo := range []string{"a/b", "c/d"} {
		if err := library.Install(context.Background(), repo, nil); err != nil {
			t.Fatal(err)
		}
	}
	library.config.Installer = fakeInstaller{err: errors.New("no network")}

	results := library.UpdateAll(context.Background(), nil)
	if len(results) != 2 {
		t.Fatalf("results = %+v, want one per source", results)
	}
	for _, result := range results {
		if result.Err == nil {
			t.Fatalf("result for %s reported success", result.Source)
		}
	}
}

// TestUpdateSkipsLocalSources checks that a folder someone made by hand has no
// repository to fetch.
func TestUpdateSkipsLocalSources(t *testing.T) {
	library := newLibrary(t, twoSkills())
	mine := filepath.Join(library.config.Root, "我的技能", "notes")
	if err := os.MkdirAll(mine, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mine, "SKILL.md"), []byte("# notes"), 0o600); err != nil {
		t.Fatal(err)
	}
	library.rebuild()

	if results := library.UpdateAll(context.Background(), nil); len(results) != 0 {
		t.Fatalf("results = %+v, want none: a local source has nothing to fetch", results)
	}
}

// TestUpdateStopsWhenCancelled checks that stopping leaves the source in flight
// as it was.
func TestUpdateStopsWhenCancelled(t *testing.T) {
	library := newLibrary(t, twoSkills())
	if err := library.Install(context.Background(), "a/b", nil); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	library.config.Installer = blockingInstaller{started: started}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan []domain.UpdateResult, 1)
	go func() { done <- library.UpdateAll(ctx, nil) }()
	<-started
	cancel()
	results := <-done

	if len(results) != 1 || !errors.Is(results[0].Err, context.Canceled) {
		t.Fatalf("results = %+v, want the source reported as cancelled", results)
	}
	if sources := library.Sources(); len(sources) != 1 || len(sources[0].Skills) != 2 {
		t.Fatalf("sources = %+v, want the source untouched", sources)
	}
}

// TestUpdateReportsProgress checks what the status line is told: which source,
// how many are done, and how many there are.
func TestUpdateReportsProgress(t *testing.T) {
	library := newLibrary(t, twoSkills())
	for _, repo := range []string{"a/b", "c/d"} {
		if err := library.Install(context.Background(), repo, nil); err != nil {
			t.Fatal(err)
		}
	}
	var seen []domain.UpdateProgress
	library.UpdateAll(context.Background(), func(progress domain.UpdateProgress) {
		seen = append(seen, progress)
	})
	if len(seen) == 0 {
		t.Fatal("the update reported nothing")
	}
	for _, progress := range seen {
		if progress.Total != 2 || progress.Source == "" {
			t.Fatalf("progress = %+v, want two sources and a name", progress)
		}
	}
	if seen[len(seen)-1].Done != 1 {
		t.Fatalf("last progress = %+v, want the second source", seen[len(seen)-1])
	}
}

// TestNewClearsTheLeftoversOfASwap checks that an update interrupted between its
// two renames does not leave folders in the library for good.
func TestNewClearsTheLeftoversOfASwap(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "skills")
	var leftovers []string
	for _, name := range []string{".staging-1", ".update-2", ".previous-3"} {
		path := filepath.Join(root, name)
		leftovers = append(leftovers, path)
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	library, err := New(Config{
		Root:      root,
		Tmp:       filepath.Join(dir, ".tmp"),
		DBPath:    filepath.Join(dir, "sqlite.db"),
		Installer: twoSkills(),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { library.Close() })

	for _, path := range leftovers {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s was left behind", filepath.Base(path))
		}
	}
}
