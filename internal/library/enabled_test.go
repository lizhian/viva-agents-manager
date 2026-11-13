package library

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestEnabledLinksSurviveReopening(t *testing.T) {
	l := newLibrary(t, twoSkills())
	if err := l.Install(context.Background(), "a/b", nil); err != nil {
		t.Fatal(err)
	}
	if err := l.SetEnabled("a@b", "tdd", true); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(l.config.EnabledDir, "vam@a@b@tdd")
	if target, err := os.Readlink(link); err != nil || target != filepath.Join(l.config.Root, "a@b", "tdd") {
		t.Fatalf("target = %q, %v", target, err)
	}
	if data, err := os.ReadFile(filepath.Join(link, "SKILL.md")); err != nil || string(data) != "# tdd" {
		t.Fatalf("linked document = %q, %v", data, err)
	}
	reopened, err := New(l.config)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	skills := reopened.Sources()[0].Skills
	if skills[0].Enabled || !skills[1].Enabled {
		t.Fatalf("reopened skills = %+v", skills)
	}
	if err := reopened.SetEnabled("a@b", "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("link remains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(l.config.Root, "a@b", "tdd", "SKILL.md")); err != nil {
		t.Fatal("disabling removed installed skill:", err)
	}
}

func TestEnabledConflictsKeepOriginalContent(t *testing.T) {
	for _, kind := range []string{"file", "directory", "link", "dangling-link"} {
		t.Run(kind, func(t *testing.T) {
			l := newLibrary(t, twoSkills())
			if err := l.Install(context.Background(), "a/b", nil); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(l.config.EnabledDir, 0o700); err != nil {
				t.Fatal(err)
			}
			conflict := l.linkPath("a@b", "tdd")
			var err error
			switch kind {
			case "file":
				err = os.WriteFile(conflict, []byte("mine"), 0o600)
			case "directory":
				err = os.Mkdir(conflict, 0o700)
			case "link":
				err = os.Symlink(l.config.Root, conflict)
			case "dangling-link":
				err = os.Symlink(filepath.Join(l.config.Root, "missing"), conflict)
			}
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.Lstat(conflict)
			if err != nil {
				t.Fatal(err)
			}
			for _, on := range []bool{true, false} {
				if err := l.SetEnabled("a@b", "", on); err == nil {
					t.Fatal("conflict not reported")
				}
				after, err := os.Lstat(conflict)
				if err != nil || !os.SameFile(before, after) {
					t.Fatal("conflict was changed")
				}
				skills := l.Sources()[0].Skills
				if skills[1].Enabled || skills[0].Enabled != on {
					t.Fatalf("actual states = %+v", skills)
				}
			}
		})
	}
}

func TestUpdateRetainsEnabledSkillsAndPrunesRemovedLinks(t *testing.T) {
	l := newLibrary(t, twoSkills())
	if err := l.Install(context.Background(), "a/b", nil); err != nil {
		t.Fatal(err)
	}
	if err := l.SetEnabled("a@b", "", true); err != nil {
		t.Fatal(err)
	}
	l.config.Installer = fakeInstaller{skills: []fakeSkill{
		{Name: "tdd", Files: map[string]string{"SKILL.md": "updated"}},
		{Name: "new-skill", Files: map[string]string{"SKILL.md": "new"}},
	}}
	results := l.UpdateAll(context.Background(), nil)
	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("update = %+v", results)
	}
	skills := l.Sources()[0].Skills
	if skills[0].Name != "new-skill" || skills[0].Enabled || !skills[1].Enabled {
		t.Fatalf("updated states = %+v", skills)
	}
	if _, err := os.Lstat(l.linkPath("a@b", "code-review")); !os.IsNotExist(err) {
		t.Fatalf("removed skill link remains: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(l.linkPath("a@b", "tdd"), "SKILL.md")); err != nil || string(data) != "updated" {
		t.Fatalf("updated linked content = %q, %v", data, err)
	}
}
