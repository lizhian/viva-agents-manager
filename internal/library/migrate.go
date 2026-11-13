package library

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lizhian/viva-agents-manager/internal/domain"
)

const legacySeparator = "_@_"

// migrateNames runs before the index is rebuilt. Preflight each source before
// changing it; conflicts are errors, never permission to replace user data.
func (l *Library) migrateNames() error {
	entries, err := os.ReadDir(l.config.Root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		old := entry.Name()
		if !entry.IsDir() || strings.HasPrefix(old, ".") {
			continue
		}
		name := old
		if !strings.Contains(old, "@") || strings.Contains(old, legacySeparator) {
			for _, sep := range []string{legacySeparator, "___"} {
				if !strings.Contains(old, sep) {
					continue
				}
				repo := strings.ReplaceAll(old, sep, "/")
				if !domain.ValidRepo(repo) {
					return fmt.Errorf("来源 %q 无法迁移：无法还原仓库名", old)
				}
				name = domain.SafeName(repo)
				break
			}
		}
		if !domain.ValidSourceName(name) {
			return fmt.Errorf("来源 %q 不可用：@ 为保留分隔符", old)
		}
		skills := scanSkills(filepath.Join(l.config.Root, old))
		for _, skill := range skills {
			if !domain.ValidSkillName(skill.Name) {
				return fmt.Errorf("技能 %q 不可用：@ 为保留分隔符", skill.Name)
			}
		}
		if err := l.migrateSource(old, name, skills); err != nil {
			return fmt.Errorf("迁移来源 %s 失败，保留冲突内容：%w", old, err)
		}
	}
	return nil
}

type migratedLink struct {
	old, new, target string
	created          bool
}

func (l *Library) migrateSource(old, name string, skills []domain.Skill) error {
	final := filepath.Join(l.config.Root, name)
	if old != name {
		if _, err := os.Lstat(final); err == nil {
			return fmt.Errorf("目标路径已存在：%s", final)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	// Also finish cleanup after interruption between the directory rename and
	// removing the old links. Their expected target still uses the old safe name.
	var links []migratedLink
	if l.config.EnabledDir != "" {
		for _, sep := range []string{legacySeparator, "___"} {
			legacyName := strings.ReplaceAll(name, "@", sep)
			for _, skill := range skills {
				previous := filepath.Join(l.config.EnabledDir, "vam"+sep+legacyName+sep+skill.Name)
				target, err := os.Readlink(previous)
				if err != nil {
					continue
				}
				if !filepath.IsAbs(target) {
					target = filepath.Join(filepath.Dir(previous), target)
				}
				if filepath.Clean(target) != filepath.Join(l.config.Root, legacyName, skill.Name) {
					continue
				}
				next := l.linkPath(name, skill.Name)
				desired := filepath.Join(final, skill.Name)
				if _, err := os.Lstat(next); err == nil {
					if !l.ownsLink(name, skill.Name) {
						return fmt.Errorf("目标路径已存在：%s", next)
					}
				} else if !os.IsNotExist(err) {
					return err
				}
				links = append(links, migratedLink{old: previous, new: next, target: desired})
			}
		}
	}
	rollback := func() {
		for _, link := range links {
			if link.created {
				_ = os.Remove(link.new)
			}
		}
	}
	for i := range links {
		link := &links[i]
		if l.ownsLink(name, filepath.Base(link.target)) {
			continue
		}
		if err := os.Symlink(link.target, link.new); err != nil {
			rollback()
			return err
		}
		link.created = true
	}
	if old != name {
		if err := os.Rename(filepath.Join(l.config.Root, old), final); err != nil {
			rollback()
			return err
		}
	}
	for _, link := range links {
		if err := os.Remove(link.old); err != nil {
			return err
		}
	}
	return nil
}
