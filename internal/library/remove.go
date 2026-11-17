package library

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lizhian/viva-agents-manager/internal/domain"
)

// Remove deletes one source and only the enabled links owned by this library.
func (l *Library) Remove(name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !domain.ValidSourceName(name) {
		return domain.ErrNoSkills
	}
	var selected *domain.Source
	for _, source := range l.scan() {
		if source.Name == name {
			selected = &source
			break
		}
	}
	if selected == nil {
		return domain.ErrNoSkills
	}
	for _, skill := range selected.Skills {
		if l.ownsLink(name, skill.Name) {
			if err := os.Remove(l.linkPath(name, skill.Name)); err != nil {
				return err
			}
		}
	}
	if err := os.RemoveAll(filepath.Join(l.config.Root, name)); err != nil {
		return err
	}
	l.rebuild()
	if l.db != nil {
		if _, err := l.db.Exec(`DELETE FROM expired_skills WHERE source=?`, name); err != nil {
			return fmt.Errorf("来源已删除，清理过期标记失败: %w", err)
		}
	}
	return nil
}
