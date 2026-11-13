package library

import (
	"fmt"

	"github.com/lizhian/viva-agents-manager/internal/domain"
)

// expired reads user metadata, not the disposable directory index. A failure
// must be reported before enabling anything, so a missing database cannot
// silently bypass an existing expiration mark.
func (l *Library) expired(source, skill string) (bool, error) {
	if l.db == nil {
		return false, fmt.Errorf("无法读取过期标记：数据库不可用")
	}
	var expired bool
	err := l.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM expired_skills WHERE source=? AND skill=?)`, source, skill).Scan(&expired)
	return expired, err
}

// SetExpired first disables the skill, then persists the user's mark. If saving
// fails the skill stays off. Clearing a mark never recreates a link.
func (l *Library) SetExpired(source, skill string, expired bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !domain.ValidSourceName(source) || !domain.ValidSkillName(skill) {
		return domain.ErrNoSkills
	}
	if l.db == nil {
		return fmt.Errorf("无法保存过期标记：数据库不可用")
	}
	found := false
	for _, item := range l.scan() {
		if item.Name != source {
			continue
		}
		for _, candidate := range item.Skills {
			if candidate.Name == skill {
				found = true
			}
		}
	}
	if !found {
		return domain.ErrNoSkills
	}
	if expired {
		if l.config.EnabledDir != "" {
			if err := l.setLink(source, skill, false); err != nil {
				return err
			}
		}
		_, err := l.db.Exec(`INSERT OR IGNORE INTO expired_skills(source,skill) VALUES(?,?)`, source, skill)
		return err
	}
	_, err := l.db.Exec(`DELETE FROM expired_skills WHERE source=? AND skill=?`, source, skill)
	return err
}
