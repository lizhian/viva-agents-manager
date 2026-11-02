package domain

import (
	"errors"
	"strings"
	"time"
)

// ErrEmptyRepo is what Install returns when it is given nothing to install.
var ErrEmptyRepo = errors.New("domain: empty repository")

// Memory is a Store kept in the process. It stands in until the application
// reads real skill directories and repositories, and it is what the tests use.
type Memory struct {
	sources []Source
	now     func() time.Time
}

// NewMemory returns a store holding the placeholder sources the interface
// started with.
func NewMemory() *Memory {
	return &Memory{
		now: time.Now,
		sources: []Source{
			{Label: "开发", Skills: []Skill{
				{Name: "building-with-viva"}, {Name: "code-review"}, {Name: "tdd"},
			}},
			{Label: "排查", Skills: []Skill{
				{Name: "diagnosing-bugs"},
			}},
			{Label: "调研与写作", Skills: []Skill{
				{Name: "research"}, {Name: "writing-for-agents"},
			}},
		},
	}
}

// Sources lists the sources in the order they were added.
func (m *Memory) Sources() []Source { return m.sources }

// Install records the repository as a source. Nothing is downloaded yet: the
// dialog only collects where a skill would come from.
func (m *Memory) Install(repo string) error {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return ErrEmptyRepo
	}
	for _, source := range m.sources {
		if source.Repo == repo {
			return errors.New("domain: repository already installed")
		}
	}
	m.sources = append(m.sources, Source{Label: repo, Repo: repo, InstalledAt: m.now()})
	return nil
}

// UpdateAll refreshes every source that has a repository. Nothing is fetched
// yet, so there is nothing to do.
func (m *Memory) UpdateAll() error { return nil }

// OpenFolder asks the desktop to show where skills live. It is a no-op until the
// application knows that folder.
func (m *Memory) OpenFolder() error { return nil }
