package shell

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lizhian/viva-agents-manager/internal/domain"
	"github.com/lizhian/viva-agents-manager/internal/instructions"
	"github.com/lizhian/viva-agents-manager/internal/pages/agentsmd"
	"github.com/vivaui/viva/headless"
)

func TestInstructionWorkspaceInShell(t *testing.T) {
	root := t.TempDir()
	config := instructions.Config{Root: filepath.Join(root, "library"), Target: filepath.Join(root, "agents", "AGENTS.md")}
	store, err := instructions.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	store.Initialize()
	id := store.Data.Add("中文回复", "# 回复规范\n\n使用中文回答。")
	store.Data.Snippet(id).Enabled = true
	store.Generate()
	window := NewWithOptions(domain.NewMemory(), agentsmd.Options{Storage: config})
	session, err := headless.Open(window.Config(), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.ClickLabel("AGENTS.md"); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{false, true} {
		if dark {
			session.ClickLabel("切换到深色主题")
		}
		for _, width := range []int{1100, 720} {
			if err := session.Resize(width, 720); err != nil {
				t.Fatal(err)
			}
			for _, problem := range headless.CheckLayout(session.Snapshot()) {
				t.Errorf("width=%d dark=%t: %+v", width, dark, problem)
			}
			if dir := os.Getenv("VAM_AGENTSMd_SHOTS"); dir != "" {
				name := "shell-light"
				if dark {
					name = "shell-dark"
				}
				if width == 720 {
					name += "-narrow"
				}
				if err := session.Screenshot(filepath.Join(dir, name+".png")); err != nil {
					t.Fatal(err)
				}
				if width == 1100 {
					if err := session.ClickLabel("分栏"); err != nil {
						t.Fatal(err)
					}
					if err := session.Screenshot(filepath.Join(dir, name+"-split.png")); err != nil {
						t.Fatal(err)
					}
					if err := session.ClickLabel("源码"); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
	}
	if err := session.ClickLabel("启用片段 中文回复"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(config.Target)
	if err != nil || len(b) != 0 {
		t.Fatalf("shell switch failed: %s %v", b, err)
	}
}
