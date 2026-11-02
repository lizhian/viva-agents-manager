package domain

import (
	"context"
	"strings"
	"time"
)

// Memory is a Store kept in the process. It is what the interface's tests drive,
// and what the application falls back to when the library cannot be opened.
type Memory struct {
	sources []Source
	now     func() time.Time
}

// NewMemory returns a store holding the placeholder sources the interface
// started with, so the tree has something to draw before a library exists.
func NewMemory() *Memory {
	return &Memory{
		now: time.Now,
		sources: []Source{
			{Name: "开发", Skills: []Skill{
				{Name: "building-with-viva"}, {Name: "code-review"}, {Name: "tdd"},
			}},
			{Name: "排查", Skills: []Skill{
				{Name: "diagnosing-bugs"},
			}},
			{Name: "调研与写作", Skills: []Skill{
				{Name: "research"}, {Name: "writing-for-agents"},
			}},
		},
	}
}

// Sources lists the sources in the order they were added.
func (m *Memory) Sources() []Source { return m.sources }

// Install records the repository as a source, naming its directory the way the
// library would. Nothing is downloaded: this is the fake.
func (m *Memory) Install(ctx context.Context, repo string, progress func(Progress)) error {
	repo = strings.TrimSpace(repo)
	switch {
	case repo == "":
		return ErrEmptyRepo
	case !ValidRepo(repo):
		return ErrBadRepo
	}
	for _, source := range m.sources {
		if source.Repo == repo {
			return ErrAlreadyInstalled
		}
	}
	if progress != nil {
		progress(ProgressDone)
	}
	m.sources = append(m.sources, Source{Name: SafeName(repo), Repo: repo, InstalledAt: m.now()})
	return nil
}

// Document reads one skill's document. The memory store has no files, so it
// hands back a short one of its own for any skill it holds.
func (m *Memory) Document(source, skill string) (Document, error) {
	for _, item := range m.sources {
		if item.Name != source {
			continue
		}
		for _, candidate := range item.Skills {
			if candidate.Name != skill {
				continue
			}
			return Document{
				Name:        skill,
				Description: "内存存储里的示例技能。",
				Body:        "# " + skill + "\n\n内存存储没有真实的文档。\n",
			}, nil
		}
	}
	return Document{}, ErrNoDocument
}

// OpenFolder reports that this store has no folder to show.
func (m *Memory) OpenFolder() error { return ErrUnsupported }
