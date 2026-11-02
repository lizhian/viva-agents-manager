package library

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lizhian/viva-agents-manager/internal/domain"
)

// maxDocumentSize bounds what the window thread reads and parses in one go: a
// skill's document is documentation, and one this large is not one.
const maxDocumentSize = 4 << 20

// Document reads one skill's SKILL.md and what it declares about itself.
//
// The names come from the tree, which came from the scan, but they are checked
// again here: a document is the one place a path is built from two names.
func (l *Library) Document(source, skill string) (domain.Document, error) {
	if !domain.ValidSkillName(source) || !domain.ValidSkillName(skill) {
		return domain.Document{}, domain.ErrNoDocument
	}
	path := filepath.Join(l.config.Root, source, skill, "SKILL.md")
	info, err := os.Stat(path)
	if err != nil {
		return domain.Document{}, fmt.Errorf("读取 %s 失败: %w", path, err)
	}
	if info.Size() > maxDocumentSize {
		return domain.Document{}, fmt.Errorf("%s 超过 %d MiB，没有打开", path, maxDocumentSize>>20)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.Document{}, fmt.Errorf("读取 %s 失败: %w", path, err)
	}
	return domain.ParseDocument(data, skill), nil
}
