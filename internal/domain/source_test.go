package domain

import "testing"

// TestSafeNameRoundTrip checks that a directory name reads back as the
// repository it was made from, which is what lets the library trust its folders.
func TestSafeNameRoundTrip(t *testing.T) {
	for _, repo := range []string{"mattpocock/skills", "a/b", "vercel-labs/agent-skills", "owner_1/repo.name"} {
		if got := RepoOf(SafeName(repo)); got != repo {
			t.Fatalf("RepoOf(SafeName(%q)) = %q", repo, got)
		}
	}
}

// TestRepoOfLocal checks that a directory name which is not a repository reads
// back as a local source rather than as something half-decoded.
func TestRepoOfLocal(t *testing.T) {
	for _, name := range []string{"mattpocock_skills", "开发", "a_@_b_@_c", "_@_"} {
		if got := RepoOf(name); got != "" {
			t.Fatalf("RepoOf(%q) = %q, want a local source", name, got)
		}
	}
}

// TestValidRepo checks what the install dialog lets through.
func TestValidRepo(t *testing.T) {
	for _, repo := range []string{"a/b", "mattpocock/skills", "A-1_b.c/d-e"} {
		if !ValidRepo(repo) {
			t.Fatalf("%q was refused", repo)
		}
	}
	for _, repo := range []string{"", "a", "a/b/c", "/b", "a/", "../b", "a/..", "a b/c", "a/b "} {
		if ValidRepo(repo) {
			t.Fatalf("%q was accepted", repo)
		}
	}
}

// TestValidSkillName checks what an installer may name a skill.
func TestValidSkillName(t *testing.T) {
	for _, name := range []string{"tdd", "Convex Best Practices", "a.b-c_d"} {
		if !ValidSkillName(name) {
			t.Fatalf("%q was refused", name)
		}
	}
	for _, name := range []string{"", ".", "..", "a/b", "/a", "../a"} {
		if ValidSkillName(name) {
			t.Fatalf("%q was accepted", name)
		}
	}
}
