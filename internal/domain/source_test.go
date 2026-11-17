package domain

import (
	"strings"
	"testing"
)

// TestSafeNameRoundTrip checks that a directory name reads back as the
// repository it was made from, which is what lets the library trust its folders.
func TestSafeNameRoundTrip(t *testing.T) {
	for _, repo := range []string{"mattpocock/skills", "a/b", "vercel-labs/agent-skills", "owner_1/repo.name", "a_/b", "a/_b", "a/b_", "a/b___c"} {
		if got := RepoOf(SafeName(repo)); got != repo {
			t.Fatalf("RepoOf(SafeName(%q)) = %q", repo, got)
		}
	}
}

// TestRepoOfLocal checks that a directory name which is not a repository reads
// back as a local source rather than as something half-decoded.
func TestRepoOfLocal(t *testing.T) {
	for _, name := range []string{"mattpocock_skills", "开发", "a@b@c", "@"} {
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
	for _, repo := range []string{"", "a", "a/b/c", "/b", "a/", "../b", "a/..", "a b/c", "a/b ", "a/b@c", "a@b/c"} {
		if ValidRepo(repo) {
			t.Fatalf("%q was accepted", repo)
		}
	}
}

// TestValidSkillName checks what an installer may name a skill.
func TestValidSkillName(t *testing.T) {
	for _, name := range []string{"tdd", "Convex Best Practices", "a.b-c_d", "_skill", "a___b"} {
		if !ValidSkillName(name) {
			t.Fatalf("%q was refused", name)
		}
	}
	for _, name := range []string{"", ".", "..", "a/b", "/a", "../a", "a@b"} {
		if ValidSkillName(name) {
			t.Fatalf("%q was accepted", name)
		}
	}
}

func TestInstallAddresses(t *testing.T) {
	for input, want := range map[string]string{
		" a/b ":                                  "a/b",
		"https://github.com/a/b":                 "a/b",
		"http://github.com/a/b.git/":             "a/b",
		"https://github.com/a/b/tree/main":       "https://github.com/a/b/tree/main",
		"https://github.com/a/b?ref=main":        "https://github.com/a/b?ref=main",
		"HTTPS://ARCHIFY.SI":                     "https://archify.si",
		"http://localhost:8080/skills?q=1#skill": "http://localhost:8080/skills?q=1#skill",
	} {
		got, err := NormalizeSource(input)
		if err != nil || got != want {
			t.Errorf("NormalizeSource(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	for _, input := range []string{"https://", "https:///a", "ftp://a.test", "https://user:pass@a.test", "npx skills add https://archify.si", "https://a.test/a b", "../x"} {
		if ValidInstallSource(input) {
			t.Errorf("accepted invalid source %q", input)
		}
	}
	base := "https://a.test/path?q=1#one"
	for _, other := range []string{"http://a.test/path?q=1#one", "https://a.test/other?q=1#one", "https://a.test/path?q=2#one", "https://a.test/path?q=1#two"} {
		if SafeName(base) == SafeName(other) {
			t.Errorf("merged %q and %q", base, other)
		}
	}
	if SafeName("https://github.com/a/b") != SafeName("a/b") {
		t.Fatal("GitHub root was not deduplicated")
	}
}

func TestReadableURLNames(t *testing.T) {
	for _, address := range []string{"https://archify.si", "https://archify.si/", "HTTPS://ARCHIFY.SI/"} {
		if got := SafeName(address); got != "archify.si" {
			t.Errorf("SafeName(%q) = %q", address, got)
		}
	}
	for _, address := range []string{
		"http://archify.si", "https://archify.si/docs/skills", "https://archify.si?q=1",
		"https://archify.si#one", "https://archify.si:8443/skills", "https://[::1]:8443/skills",
		"https://archify.si/" + strings.Repeat("long-", 100),
	} {
		name := SafeName(address)
		if len(name) > 170 || !ValidSourceName(name) || strings.ContainsAny(name, "/@:") || strings.HasPrefix(name, ".") {
			t.Errorf("unsafe or unbounded name %q for %q", name, address)
		}
		if strings.Contains(address, "archify.si") && !strings.HasPrefix(name, "archify.si--") {
			t.Errorf("domain missing: %q", name)
		}
		if !MatchesSourceName(name, address) {
			t.Errorf("new name not recognized: %s", name)
		}
	}
	if SafeName("https://archify.si/a/b") == SafeName("https://archify.si/a-b") {
		t.Fatal("cleaned paths collided")
	}
}
