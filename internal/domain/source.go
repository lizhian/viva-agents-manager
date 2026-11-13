// Package domain holds what the application manages: where skills come from,
// and later MCP servers and instruction documents. It knows nothing about the
// interface, so it is testable without a window.
package domain

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Skill is one skill a source provides, named as its directory is.
type Skill struct {
	Name    string
	Enabled bool
	Expired bool
}

// Source is where skills come from. Name is its directory in the library; Repo
// is the repository it was installed from, empty for a source that is only a
// folder on this machine.
type Source struct {
	Name        string
	Repo        string
	InstalledAt time.Time
	Skills      []Skill
}

// Label is what the tree shows: the repository, or the folder's own name for a
// source that has no repository.
func (s Source) Label() string {
	if s.Repo != "" {
		return s.Repo
	}
	return s.Name
}

// Match reports whether the source has a skill whose name contains query. Query
// is expected lowercase, as the search box normalizes it; names are lowercase
// too, so nothing is lowered per frame.
func (s Source) Match(query string) bool {
	for _, skill := range s.Skills {
		if skill.Match(query) {
			return true
		}
	}
	return false
}

// Match reports whether the skill's name contains query.
func (s Skill) Match(query string) bool { return strings.Contains(s.Name, query) }

// repoPattern is the shape of a repository the application accepts: one slash,
// and characters GitHub allows on either side of it.
var repoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

// ValidRepo reports whether repo names a repository as "owner/repo".
func ValidRepo(repo string) bool {
	if !repoPattern.MatchString(repo) || strings.Contains(repo, separator) {
		return false
	}
	owner, name, _ := strings.Cut(repo, "/")
	return owner != "." && owner != ".." && name != "." && name != ".."
}

// separator joins the two halves of a repository in a directory name. It is
// reversible, unlike a bare underscore: "mattpocock_skills" could be either
// "mattpocock/skills" or a folder someone named that.
const separator = "@"

// SafeName is the directory a repository's skills are installed in.
func SafeName(repo string) string { return strings.ReplaceAll(repo, "/", separator) }

// RepoOf reads a repository back out of a library directory name. A name that
// does not read back as a repository is a local source, and reports none.
func RepoOf(name string) string {
	if !strings.Contains(name, separator) {
		return ""
	}
	repo := strings.ReplaceAll(name, separator, "/")
	if !ValidRepo(repo) {
		return ""
	}
	return repo
}

// ValidSkillName accepts a path element that cannot overlap the reserved
// separator in an enabled link.
func ValidSkillName(name string) bool {
	return validPathName(name) && !strings.Contains(name, separator)
}

// ValidSourceName accepts a repository's safe name or an unambiguous local name.
func ValidSourceName(name string) bool {
	return RepoOf(name) != "" || ValidSkillName(name)
}

func validPathName(name string) bool {
	return name != "" && name != "." && name != ".." && filepath.Base(name) == name
}
