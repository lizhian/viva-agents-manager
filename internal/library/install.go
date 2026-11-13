package library

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lizhian/viva-agents-manager/internal/domain"
)

// Install fetches a repository into the temporary folder and migrates what it
// holds into the library.
//
// Nothing appears under the source's name until the migration has landed: the
// skills are copied into a staging folder inside the library, and one rename
// puts them in place, so a failure before that leaves the library as it was.
func (l *Library) Install(ctx context.Context, repo string, progress func(domain.Progress)) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	repo = strings.TrimSpace(repo)
	switch {
	case repo == "":
		return domain.ErrEmptyRepo
	case !domain.ValidRepo(repo):
		return domain.ErrBadRepo
	}
	final := filepath.Join(l.config.Root, domain.SafeName(repo))
	switch _, err := os.Lstat(final); {
	case err == nil:
		return fmt.Errorf("%w: %s", domain.ErrAlreadyInstalled, repo)
	case !os.IsNotExist(err):
		return err
	}

	temp, err := os.MkdirTemp(l.config.Tmp, "install-")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(temp)

	if progress != nil {
		progress(domain.ProgressPreparing)
	}
	prepared, err := l.config.Installer.Prepare(ctx, repo, temp)
	if err != nil {
		return err
	}
	if len(prepared) == 0 {
		return domain.ErrNoSkills
	}

	// The staging folder lives inside the library, so the rename that publishes
	// it stays on one volume and is atomic.
	stage, err := os.MkdirTemp(l.config.Root, ".staging-")
	if err != nil {
		return fmt.Errorf("创建暂存目录失败: %w", err)
	}
	defer os.RemoveAll(stage)

	if err := stageSkills(prepared, stage); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	if progress != nil {
		progress(domain.ProgressMigrating)
	}
	if err := os.Rename(stage, final); err != nil {
		return fmt.Errorf("迁移到技能库失败: %w", err)
	}
	// The skills are on disk now, which is what the application reads; a failed
	// rebuild costs a slower listing, not the install.
	l.rebuild()
	if progress != nil {
		progress(domain.ProgressDone)
	}
	return nil
}

// stageSkills copies what an installer prepared into a folder of the library's
// own, so publishing it is one rename.
func stageSkills(prepared []domain.PreparedSkill, stage string) error {
	seen := make(map[string]bool, len(prepared))
	for _, item := range prepared {
		if !domain.ValidSkillName(item.Name) {
			return fmt.Errorf("技能名 %q 不可用（@ 为保留分隔符）", item.Name)
		}
		if seen[item.Name] {
			return fmt.Errorf("技能 %s 出现了两次", item.Name)
		}
		seen[item.Name] = true
		target := filepath.Join(stage, item.Name)
		if err := copyTree(item.Path, target); err != nil {
			return fmt.Errorf("复制技能 %s 失败: %w", item.Name, err)
		}
		if _, err := os.Stat(filepath.Join(target, "SKILL.md")); err != nil {
			return fmt.Errorf("技能 %s 没有 SKILL.md", item.Name)
		}
	}
	return nil
}
