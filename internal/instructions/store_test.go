package instructions

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T) *Store {
	t.Helper()
	root := t.TempDir()
	s, err := Open(Config{Root: filepath.Join(root, "data"), Target: filepath.Join(root, "agents", "AGENTS.md")})
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func output(t *testing.T, s *Store) string {
	t.Helper()
	b, err := os.ReadFile(s.Config.Target)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func TestTakeoverRestartAndAllDisabled(t *testing.T) {
	s := fixture(t)
	if err := os.MkdirAll(filepath.Dir(s.Config.Target), 0700); err != nil {
		t.Fatal(err)
	}
	original := "# Existing\n\nkeep me\n"
	os.WriteFile(s.Config.Target, []byte(original), 0600)
	if err := s.Initialize(); err != nil {
		t.Fatal(err)
	}
	if len(s.Data.Snippets) != 1 || !s.Data.Snippets[0].Enabled || output(t, s) != original {
		t.Fatal("takeover lost existing instructions")
	}
	s, err := Open(s.Config)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Initialize(); err != nil {
		t.Fatal(err)
	}
	if len(s.Data.Snippets) != 1 {
		t.Fatal("duplicated takeover")
	}
	s.Data.Snippets[0].Enabled = false
	if err = s.Generate(); err != nil {
		t.Fatal(err)
	}
	if output(t, s) != "" {
		t.Fatal("all disabled must generate empty file")
	}
}
func TestFlatMergeOrder(t *testing.T) {
	s := fixture(t)
	if err := s.Initialize(); err != nil {
		t.Fatal(err)
	}
	a := s.Data.Add("A", "  indented\n\n")
	b := s.Data.Add("B", "B\r\n")
	s.Data.Enable(a, true)
	s.Data.Enable(b, true)
	if err := s.Generate(); err != nil {
		t.Fatal(err)
	}
	if got := output(t, s); got != "  indented\n\nB\n" {
		t.Fatalf("merge %q", got)
	}
	if err := s.Data.Move(b, a); err != nil {
		t.Fatal(err)
	}
	if err := s.Generate(); err != nil {
		t.Fatal(err)
	}
	if output(t, s) != "B\n\n  indented\n" {
		t.Fatal("manual order ignored")
	}
}

func TestExternalChangeRequiresExplicitVersionedResolution(t *testing.T) {
	for _, keep := range []bool{true, false} {
		t.Run(map[bool]string{true: "keep", false: "discard"}[keep], func(t *testing.T) {
			s := fixture(t)
			s.Initialize()
			id := s.Data.Add("local", "local")
			s.Data.Snippet(id).Enabled = true
			s.Generate()
			os.WriteFile(s.Config.Target, []byte("external"), 0600)
			if err := s.Generate(); !errors.Is(err, ErrConflict) {
				t.Fatalf("missing conflict: %v", err)
			}
			if output(t, s) != "external" {
				t.Fatal("overwrote external")
			}
			o, _ := s.Observe()
			os.WriteFile(s.Config.Target, []byte("new external"), 0600)
			if err := s.Resolve(o, keep); !errors.Is(err, ErrConflict) {
				t.Fatal("accepted stale resolution")
			}
			o, _ = s.Observe()
			if err := s.Resolve(o, keep); err != nil {
				t.Fatal(err)
			}
			if output(t, s) != "local\n" {
				t.Fatal("wrong regenerated output")
			}
			if keep {
				if len(s.Data.Snippets) != 2 || s.Data.Snippets[1].Enabled || s.Data.Snippets[1].Body != "new external" {
					t.Fatal("external import not retained disabled")
				}
			} else if len(s.Data.Snippets) != 1 {
				t.Fatal("discard imported content")
			}
		})
	}
}
func TestDeletedTargetAndPendingRecovery(t *testing.T) {
	s := fixture(t)
	s.Initialize()
	id := s.Data.Add("A", "A")
	s.Data.Snippet(id).Enabled = true
	s.Generate()
	os.Remove(s.Config.Target)
	if err := s.Generate(); !errors.Is(err, ErrConflict) {
		t.Fatal("deletion not detected")
	}
	o, _ := s.Observe()
	if err := s.Resolve(o, true); err != nil {
		t.Fatal(err)
	}
	if output(t, s) != "A\n" {
		t.Fatal("delete recovery failed")
	}
	for _, written := range []bool{false, true} {
		s.Data.Snippet(id).Body = "changed"
		s.Data.Pending = digest([]byte(s.Data.Merge()))
		s.Save()
		if written {
			os.WriteFile(s.Config.Target, []byte(s.Data.Merge()), 0600)
		}
		restarted, err := Open(s.Config)
		if err != nil {
			t.Fatal(err)
		}
		if err = restarted.Initialize(); err != nil {
			t.Fatal(err)
		}
		if output(t, restarted) != "changed\n" || restarted.Data.Pending != "" {
			t.Fatal("pending generation not recovered")
		}
		s = restarted
		s.Data.Snippet(id).Body = "A"
		s.Generate()
	}
}
func TestUnreadableLibraryNeverOverwritesTarget(t *testing.T) {
	s := fixture(t)
	os.MkdirAll(s.Config.Root, 0700)
	os.WriteFile(s.path(), []byte("bad json"), 0600)
	os.MkdirAll(filepath.Dir(s.Config.Target), 0700)
	os.WriteFile(s.Config.Target, []byte("untouched"), 0600)
	broken, err := Open(s.Config)
	if err == nil {
		t.Fatal("accepted corrupt library")
	}
	if broken.Initialize() == nil || broken.Save() == nil {
		t.Fatal("wrote corrupt library")
	}
	if output(t, s) != "untouched" {
		t.Fatal("overwrote target")
	}
}
func TestSearchAndPersistence(t *testing.T) {
	s := fixture(t)
	s.Initialize()
	id := s.Data.Add("hello", "# hello")
	n := s.Data.Snippet(id)
	n.Tags = []string{"Go"}
	if n.Enabled || n.Name != "hello" || n.Body != "# hello" || !Matches(n, "missing | go") {
		t.Fatal("bad search")
	}
	n.Enabled = true
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	restored, err := Open(s.Config)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Data.Snippet(id).Tags[0] != "Go" || !restored.Data.Snippet(id).Enabled {
		t.Fatal("lost state")
	}
}

func TestWriteFailureRetainsDurableSource(t *testing.T) {
	s := fixture(t)
	s.Initialize()
	id := s.Data.Add("A", "A")
	s.Data.Snippet(id).Enabled = true
	target := s.Config.Target
	s.Config.Target = filepath.Join(target, "invalid")
	if err := s.Generate(); err == nil {
		t.Fatal("expected failure")
	}
	restored, err := Open(s.Config)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Data.Snippet(id).Body != "A" {
		t.Fatal("failed output lost source")
	}
}
