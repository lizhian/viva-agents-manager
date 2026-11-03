package library

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lizhian/viva-agents-manager/internal/domain"
)

// maxDocumentSize bounds what the window thread reads and parses in one go: a
// skill's document is documentation, and one this large is not one.
const maxDocumentSize = 4 << 20

// Document reads one file of a skill's directory and what it declares about
// itself.
//
// The names come from the tree, which came from the scan, but they are checked
// again here: a document is the one place a path is built from names the
// application did not choose.
func (l *Library) Document(source, skill, file string) (domain.Document, error) {
	if !domain.ValidSkillName(source) || !domain.ValidSkillName(skill) {
		return domain.Document{}, domain.ErrNoDocument
	}
	dir := filepath.Join(l.config.Root, source, skill)
	path, ok := inside(dir, file)
	if !ok {
		return domain.Document{}, domain.ErrNoDocument
	}
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
	// No fallback name: only the caller knows what to call a document that
	// declares nothing, and "SKILL" is not a name anyone wants to read.
	return domain.ParseDocument(data, ""), nil
}

// inside joins file to dir, refusing anything that would leave it, links
// included.
func inside(dir, file string) (string, bool) {
	if file == "" || filepath.IsAbs(file) || strings.HasPrefix(file, "/") {
		return "", false
	}
	path := filepath.Join(dir, filepath.FromSlash(file))
	if !within(dir, path) {
		return "", false
	}
	// A link inside the skill could point out of it; the copy at install time
	// refuses those, but a folder someone made by hand has not been copied.
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path, true // missing: the read reports it
	}
	root, err := filepath.EvalSymlinks(dir)
	if err != nil || !within(root, resolved) {
		return "", false
	}
	return path, true
}
