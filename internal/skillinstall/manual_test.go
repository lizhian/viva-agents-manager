//go:build manual

package skillinstall

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestNpxFetchesARealRepository is the end-to-end check of the installer: it runs
// the real npx against a real repository, into a folder the test owns, and leaves
// the application's own library alone. It needs the network and a working Node,
// so it is behind the manual tag:
//
//	go test -tags manual -run TestNpxFetchesARealRepository ./internal/skillinstall
func TestNpxFetchesARealRepository(t *testing.T) {
	dir := t.TempDir()
	prepared, err := Npx{}.Prepare(context.Background(), "mattpocock/skills", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared) == 0 {
		t.Fatal("the repository reported no skills")
	}
	for _, skill := range prepared {
		if _, err := os.Stat(filepath.Join(skill.Path, "SKILL.md")); err != nil {
			t.Fatalf("%s has no SKILL.md: %v", skill.Name, err)
		}
	}
	t.Logf("fetched %d skills", len(prepared))
}
