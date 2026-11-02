package library

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// copyTree copies a prepared skill into the staging folder. It refuses anything
// that would make the copy reach outside the skill it came from — a link
// pointing elsewhere, a device, a pipe — because a repository is someone else's
// content and a link in it is not a reason to write outside the library.
func copyTree(src, dst string) error {
	root, err := filepath.EvalSymlinks(src)
	if err != nil {
		return err
	}
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.Type()&os.ModeSymlink != 0 {
			link, err := filepath.EvalSymlinks(path)
			if err != nil || !within(root, link) {
				return fmt.Errorf("技能内的软链 %s 指向技能之外", rel)
			}
			relative, err := filepath.Rel(filepath.Dir(path), link)
			if err != nil {
				return err
			}
			return os.Symlink(relative, target)
		}
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("技能内有无法复制的文件 %s", rel)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		return copyFile(path, target, info.Mode().Perm())
	})
}

// copyFile copies one file, keeping its permission bits.
func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

// within reports whether path is root or sits inside it.
func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
