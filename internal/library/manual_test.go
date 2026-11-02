//go:build manual

package library

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lizhian/viva-agents-manager/internal/skillinstall"
)

// TestInstallFromARealRepository is the whole chain against the real thing: npx
// fetches a repository, the library migrates it, and the index lists what landed.
// It needs the network and a working Node, so it is behind the manual tag, and it
// writes only to a folder the test owns:
//
//	go test -tags manual -run TestInstallFromARealRepository ./internal/library
func TestInstallFromARealRepository(t *testing.T) {
	dir := t.TempDir()
	store, err := New(Config{
		Root:      filepath.Join(dir, "skills"),
		Tmp:       filepath.Join(dir, ".tmp"),
		DBPath:    filepath.Join(dir, "sqlite.db"),
		Installer: skillinstall.Npx{},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	if err := store.Install(context.Background(), "mattpocock/skills", nil); err != nil {
		t.Fatal(err)
	}

	sources := store.Sources()
	if len(sources) != 1 {
		t.Fatalf("source count = %d, want 1", len(sources))
	}
	if sources[0].Repo != "mattpocock/skills" {
		t.Fatalf("source = %+v, want the repository it came from", sources[0])
	}
	if len(sources[0].Skills) == 0 {
		t.Fatal("the install reported no skills")
	}
	for _, skill := range sources[0].Skills {
		path := filepath.Join(dir, "skills", "mattpocock_@_skills", skill.Name, "SKILL.md")
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s was not migrated: %v", skill.Name, err)
		}
	}
	t.Logf("installed %d skills", len(sources[0].Skills))
}
