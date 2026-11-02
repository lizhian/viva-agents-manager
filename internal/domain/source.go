// Package domain holds what the application manages: where skills come from,
// and later MCP servers and instruction documents. It knows nothing about the
// interface, so it is testable without a window.
package domain

import (
	"strings"
	"time"
)

// Skill is one skill a source provides.
type Skill struct {
	Name        string
	Description string
	Enabled     bool
}

// Source is where skills come from: a GitHub repository, or a folder on this
// machine (Repo empty). Label is what the tree shows, so a local source can have
// a name of its own.
type Source struct {
	Label       string
	Repo        string
	InstalledAt time.Time
	Skills      []Skill
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
