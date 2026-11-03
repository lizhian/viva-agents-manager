package skillinstall

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lizhian/viva-agents-manager/internal/domain"
)

// TestFindNpx checks that npx is found by scanning the path, and reported as
// missing when it is not there.
func TestFindNpx(t *testing.T) {
	dir := t.TempDir()
	if _, err := findNpx(dir); !errors.Is(err, domain.ErrNpxMissing) {
		t.Fatalf("findNpx on an empty folder = %v, want ErrNpxMissing", err)
	}

	npx := filepath.Join(dir, "npx")
	if err := os.WriteFile(npx, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	found, err := findNpx(dir)
	if err != nil {
		t.Fatal(err)
	}
	if found != npx {
		t.Fatalf("findNpx = %q, want %q", found, npx)
	}
}

// TestParseReadsTheCLIOutput checks the JSON the CLI prints is turned into the
// skills it names.
func TestParseReadsTheCLIOutput(t *testing.T) {
	dir := t.TempDir()
	tdd := filepath.Join(dir, ".agents", "skills", "tdd")
	if err := os.MkdirAll(tdd, 0o700); err != nil {
		t.Fatal(err)
	}
	// The CLI prints its own box drawing before the JSON.
	out := []byte("◇ Installing all 1 skills\n[\n  {\"name\": \"tdd\", \"path\": \"" + tdd + "\"}\n]\n")

	prepared, err := parse(dir, out)
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared) != 1 || prepared[0].Name != "tdd" {
		t.Fatalf("prepared = %+v, want the one skill the CLI listed", prepared)
	}
}

// TestParseRefusesPathsOutsideTheFolder checks that a CLI reporting a skill
// somewhere else cannot make the application install it.
func TestParseRefusesPathsOutsideTheFolder(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	out := []byte(`[{"name": "tdd", "path": "` + outside + `"}]`)

	if _, err := parse(dir, out); err == nil {
		t.Fatal("a skill outside the temporary folder was accepted")
	}
}

// TestParseRejectsGarbage checks that output without JSON is reported rather
// than read as no skills.
func TestParseRejectsGarbage(t *testing.T) {
	if _, err := parse(t.TempDir(), []byte("not json")); err == nil {
		t.Fatal("output with no JSON was accepted")
	}
}

// TestLastLine checks that the CLI's box drawing is stripped from an error.
func TestLastLine(t *testing.T) {
	message := "╭───────────╮\n│           │\n│  ⚠ 仓库不存在  │\n│           │\n╰───────────╯\n"
	if got, want := lastLine(message), "⚠ 仓库不存在"; got != want {
		t.Fatalf("lastLine = %q, want %q", got, want)
	}
}
