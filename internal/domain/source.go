// Package domain holds what the application manages: where skills come from,
// and later MCP servers and instruction documents. It knows nothing about the
// interface, so it is testable without a window.
package domain

import (
	"crypto/sha256"
	"fmt"
	"net/url"
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
// is the remote install address (owner/repo or HTTP(S)), empty for a source that is only a
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

// NormalizeSource validates an install address and folds GitHub repository roots
// into the existing shorthand. Other URL components retain their meaning.
func NormalizeSource(input string) (string, error) {
	return normalizeSource(input, true)
}

// collapseRoot is false only when checking names written by the old URL scheme.
func normalizeSource(input string, collapseRoot bool) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", ErrEmptyRepo
	}
	if ValidRepo(input) {
		return input, nil
	}
	u, err := url.Parse(input)
	if err != nil || u.Hostname() == "" || u.User != nil || strings.ContainsAny(input, " \t\r\n") {
		return "", ErrBadRepo
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", ErrBadRepo
	}
	u.Host = strings.ToLower(u.Host)
	if u.Host == "github.com" && u.RawQuery == "" && !u.ForceQuery && u.Fragment == "" && !strings.Contains(input, "#") && u.RawPath == "" {
		repo := strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), "/"), ".git")
		if ValidRepo(repo) {
			return repo, nil
		}
	}
	if collapseRoot && u.Path == "/" && u.RawPath == "" {
		u.Path, u.RawPath = "", ""
	}
	return u.String(), nil
}

// ValidInstallSource reports whether the input names an installable remote source.
func ValidInstallSource(input string) bool { _, err := NormalizeSource(input); return err == nil }

// SafeName keeps repository names compatible and gives URLs a bounded identity.
func SafeName(source string) string {
	normalized, err := NormalizeSource(source)
	if err != nil {
		return ""
	}
	if ValidRepo(normalized) {
		return strings.ReplaceAll(normalized, "/", separator)
	}
	u, _ := url.Parse(normalized)
	host := readablePart(u.Hostname(), 80)
	if u.Scheme == "https" && u.Port() == "" && u.Path == "" && u.RawQuery == "" && !u.ForceQuery && u.Fragment == "" && host == u.Hostname() {
		return host
	}
	summary := host
	if u.Port() != "" {
		summary += "--p" + readablePart(u.Port(), 8)
	}
	if u.Scheme == "http" {
		summary += "--http"
	}
	if path := readablePart(u.Path, 48); path != "site" {
		summary += "--" + path
	}
	digest := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("%s--%x", summary, digest[:6])
}

// MatchesSourceName accepts current URL names and the legacy full-hash names.
// Legacy identities retain the original root slash when checking their hash.
func MatchesSourceName(name, address string) bool {
	if name == SafeName(address) {
		return true
	}
	legacy, err := normalizeSource(address, false)
	return err == nil && !ValidRepo(legacy) && name == fmt.Sprintf("url-%x", sha256.Sum256([]byte(legacy)))
}

// readablePart bounds a filesystem label and avoids reserved separators. The
// digest, not the cleaned label, distinguishes complex URL identities.
func readablePart(text string, limit int) string {
	var b strings.Builder
	for _, c := range strings.ToLower(text) {
		if b.Len() >= limit {
			break
		}
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '.' || c == '-' {
			b.WriteRune(c)
		} else if b.Len() > 0 {
			b.WriteByte('-')
		}
	}
	part := strings.Trim(b.String(), ".-")
	if part == "" {
		return "site"
	}
	return part
}

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
