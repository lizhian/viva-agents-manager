package library

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/lizhian/viva-agents-manager/internal/domain"
)

// linkPath names a link without changing the skill's own directory or metadata.
func (l *Library) linkPath(source, skill string) string {
	return filepath.Join(l.config.EnabledDir, "vam@"+source+"@"+skill)
}

// ownsLink accepts only a symlink to this exact library skill. A dangling link
// still belongs to us, so it can be removed when an update drops the skill.
func (l *Library) ownsLink(source, skill string) bool {
	if l.config.EnabledDir == "" {
		return false
	}
	link := l.linkPath(source, skill)
	target, err := os.Readlink(link)
	if err != nil {
		return false
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(link), target)
	}
	return filepath.Clean(target) == filepath.Join(l.config.Root, source, skill)
}

func (l *Library) withEnabled(sources []domain.Source) []domain.Source {
	reported := false
	for i := range sources {
		for j := range sources[i].Skills {
			skill := &sources[i].Skills[j]
			expired, err := l.expired(sources[i].Name, skill.Name)
			if err != nil && !reported {
				log.Printf("library: 读取过期标记失败: %v", err)
				reported = true
			}
			skill.Expired = expired
			skill.Enabled = !skill.Expired && l.ownsLink(sources[i].Name, skill.Name)
			if skill.Enabled {
				info, err := os.Stat(filepath.Join(l.config.Root, sources[i].Name, skill.Name, "SKILL.md"))
				skill.Enabled = err == nil && info.Mode().IsRegular()
			}
		}
	}
	return sources
}

// SetEnabled changes only links owned by this library, never a conflicting
// file or another program's link. Bulk changes keep successful results and
// return failures together; the next Sources call reads the actual disk state.
func (l *Library) SetEnabled(source, skill string, enabled bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.config.EnabledDir == "" {
		return domain.ErrUnsupported
	}
	if !domain.ValidSourceName(source) || (skill != "" && !domain.ValidSkillName(skill)) {
		return domain.ErrNoSkills
	}
	var selected []domain.Skill
	foundSource := false
	for _, candidate := range l.scan() {
		if candidate.Name != source {
			continue
		}
		foundSource = true
		for _, item := range candidate.Skills {
			if skill == "" || item.Name == skill {
				if enabled {
					expired, err := l.expired(source, item.Name)
					if err != nil {
						return err
					}
					if expired {
						if skill != "" {
							return domain.ErrExpired
						}
						continue
					}
				}
				selected = append(selected, item)
			}
		}
	}
	if len(selected) == 0 {
		if skill == "" && foundSource {
			return nil
		}
		return domain.ErrNoSkills
	}
	if enabled {
		if err := os.MkdirAll(l.config.EnabledDir, 0o700); err != nil {
			return err
		}
	}
	var failures []error
	for _, item := range selected {
		if err := l.setLink(source, item.Name, enabled); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", item.Name, err))
		}
	}
	return errors.Join(failures...)
}

func (l *Library) setLink(source, skill string, enabled bool) error {
	link := l.linkPath(source, skill)
	_, err := os.Lstat(link)
	if os.IsNotExist(err) {
		if !enabled {
			return nil
		}
		return os.Symlink(filepath.Join(l.config.Root, source, skill), link)
	}
	if err != nil {
		return err
	}
	if !l.ownsLink(source, skill) {
		return fmt.Errorf("路径已被占用，保留原内容：%s", link)
	}
	if enabled {
		return nil
	}
	return os.Remove(link)
}

// pruneEnabled removes only owned links for skills a successful update removed.
func (l *Library) pruneEnabled(source domain.Source) error {
	if l.config.EnabledDir == "" {
		return nil
	}
	var failures []error
	for _, skill := range source.Skills {
		if _, err := os.Stat(filepath.Join(l.config.Root, source.Name, skill.Name, "SKILL.md")); !os.IsNotExist(err) {
			continue
		}
		if l.ownsLink(source.Name, skill.Name) {
			if err := os.Remove(l.linkPath(source.Name, skill.Name)); err != nil {
				failures = append(failures, err)
			}
		}
	}
	return errors.Join(failures...)
}
