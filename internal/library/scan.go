package library

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lizhian/viva-agents-manager/internal/domain"
)

// scan reads the library folder: every directory in it is a source, and every
// directory inside that holds a SKILL.md is one of its skills. A source with no
// skill is left out, because there is nothing the tree could show for it.
//
// A name that does not read back as a repository is a local source, which is how
// a folder someone put here by hand stays visible.
func (l *Library) scan() []domain.Source {
	entries, err := os.ReadDir(l.config.Root)
	if err != nil {
		return nil
	}
	sources := make([]domain.Source, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		skills := scanSkills(filepath.Join(l.config.Root, entry.Name()))
		if len(skills) == 0 {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		sources = append(sources, domain.Source{
			Name:        entry.Name(),
			Repo:        domain.RepoOf(entry.Name()),
			InstalledAt: info.ModTime(),
			Skills:      skills,
		})
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].Name < sources[j].Name })
	return sources
}

// scanSkills lists the skills in one source's directory. ReadDir sorts by name,
// so the leaves come out in the order the tree draws them.
func scanSkills(dir string) []domain.Skill {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	skills := make([]domain.Skill, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, entry.Name(), "SKILL.md")); err != nil {
			continue
		}
		skills = append(skills, domain.Skill{Name: entry.Name()})
	}
	return skills
}
