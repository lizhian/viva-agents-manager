package domain

import (
	"context"
	"errors"
	"testing"
)

// TestMemoryInstall checks that a repository becomes a source of its own.
func TestMemoryInstall(t *testing.T) {
	store := NewMemory()
	before := len(store.Sources())

	if err := store.Install(context.Background(), " anthropics/skills ", nil); err != nil {
		t.Fatal(err)
	}
	sources := store.Sources()
	if len(sources) != before+1 {
		t.Fatalf("source count = %d, want %d", len(sources), before+1)
	}
	last := sources[len(sources)-1]
	if last.Repo != "anthropics/skills" || last.Label() != "anthropics/skills" {
		t.Fatalf("installed source = %+v, want the trimmed repository", last)
	}
	if last.Name != "anthropics@skills" {
		t.Fatalf("installed directory = %q, want the safe name", last.Name)
	}
	if last.InstalledAt.IsZero() {
		t.Fatal("the installed source has no installation time")
	}
}

// TestMemoryInstallRejects checks the ways installing fails.
func TestMemoryInstallRejects(t *testing.T) {
	store := NewMemory()
	if err := store.Install(context.Background(), "   ", nil); !errors.Is(err, ErrEmptyRepo) {
		t.Fatalf("installing an empty repository = %v, want ErrEmptyRepo", err)
	}
	if err := store.Install(context.Background(), "a/b/c", nil); !errors.Is(err, ErrBadRepo) {
		t.Fatalf("installing a path = %v, want ErrBadRepo", err)
	}
	if err := store.Install(context.Background(), "a/b", nil); err != nil {
		t.Fatal(err)
	}
	if err := store.Install(context.Background(), "a/b", nil); !errors.Is(err, ErrAlreadyInstalled) {
		t.Fatalf("installing the same repository twice = %v, want ErrAlreadyInstalled", err)
	}
}

// TestSourceMatch checks the search predicate the tree filters with.
func TestSourceMatch(t *testing.T) {
	source := Source{Name: "开发", Skills: []Skill{{Name: "code-review"}, {Name: "tdd"}}}

	if !source.Match("") {
		t.Fatal("an empty query matched nothing")
	}
	if !source.Match("tdd") {
		t.Fatal("tdd did not match")
	}
	if source.Match("research") {
		t.Fatal("a skill the source does not have matched")
	}
	if source.Match("开发") {
		t.Fatal("a branch label matched, but only skill names should")
	}
}
