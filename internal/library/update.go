package library

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/lizhian/viva-agents-manager/internal/domain"
)

// UpdateAll fetches every source that has a repository again and replaces what
// the library holds with it.
//
// A source is replaced whole, so a skill the repository no longer has is gone
// afterwards: what the library holds and what the repository holds are the same
// thing. A source that fails keeps what it had, and the rest still update.
func (l *Library) UpdateAll(ctx context.Context, progress func(domain.UpdateProgress)) []domain.UpdateResult {
	l.mu.Lock()
	defer l.mu.Unlock()

	var sources []domain.Source
	for _, source := range l.scan() {
		if source.Repo != "" {
			sources = append(sources, source)
		}
	}
	results := make([]domain.UpdateResult, 0, len(sources))
	for i, source := range sources {
		if ctx.Err() != nil {
			return results
		}
		report := func(stage domain.Progress) {
			if progress != nil {
				progress(domain.UpdateProgress{
					Source: source.Repo, Done: i, Total: len(sources), Stage: stage,
				})
			}
		}
		report(domain.ProgressPreparing)
		results = append(results, domain.UpdateResult{Source: source.Repo, Err: l.replace(ctx, source, report)})
	}
	return results
}

// replace fetches a source again and swaps its folder for what came back.
func (l *Library) replace(ctx context.Context, source domain.Source, progress func(domain.Progress)) error {
	final := filepath.Join(l.config.Root, source.Name)
	temp, err := os.MkdirTemp(l.config.Tmp, "update-")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(temp)

	prepared, err := l.config.Installer.Prepare(ctx, source.Repo, temp)
	if err != nil {
		return err
	}
	if len(prepared) == 0 {
		return domain.ErrNoSkills
	}
	stage, err := os.MkdirTemp(l.config.Root, ".update-")
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

	// The old folder is moved aside rather than removed, so a swap that fails
	// half way can put back exactly what was there.
	previous, err := os.MkdirTemp(l.config.Root, ".previous-")
	if err != nil {
		return fmt.Errorf("创建备份目录失败: %w", err)
	}
	if err := os.Remove(previous); err != nil {
		return fmt.Errorf("创建备份目录失败: %w", err)
	}
	if err := os.Rename(final, previous); err != nil {
		return fmt.Errorf("备份旧版本失败: %w", err)
	}
	if err := os.Rename(stage, final); err != nil {
		if restore := os.Rename(previous, final); restore != nil {
			return fmt.Errorf("写入新版本失败: %v; 恢复旧版本也失败: %w", err, restore)
		}
		return fmt.Errorf("写入新版本失败: %w", err)
	}
	// What is on disk is what the application reads; a failed rebuild costs a
	// slower listing, not the update.
	l.rebuild()
	if err := os.RemoveAll(previous); err != nil {
		log.Printf("library: 删除旧版本失败: %v", err)
	}
	if progress != nil {
		progress(domain.ProgressDone)
	}
	if err := l.pruneEnabled(source); err != nil {
		return fmt.Errorf("技能已更新，清理已移除技能的软链失败: %w", err)
	}
	return nil
}
