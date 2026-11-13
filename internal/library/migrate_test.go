package library

import (
	"os"
	"path/filepath"
	"testing"
)

func legacyConfig(t *testing.T, sep string) Config {
	t.Helper()
	dir := t.TempDir()
	config := Config{Root: filepath.Join(dir, "skills"), Tmp: filepath.Join(dir, "tmp"), DBPath: filepath.Join(dir, "index.db"), EnabledDir: filepath.Join(dir, "enabled"), Installer: twoSkills()}
	target := filepath.Join(config.Root, "a"+sep+"b", "tdd")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(config.EnabledDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(config.EnabledDir, "vam"+sep+"a"+sep+"b"+sep+"tdd")); err != nil {
		t.Fatal(err)
	}
	return config
}

func TestStartupMigratesNamesAndEnabledLinks(t *testing.T) {
	for _, sep := range []string{"_@_", "___"} {
		t.Run(sep, func(t *testing.T) {
			for _, interrupted := range []bool{false, true} {
				t.Run(map[bool]string{false: "legacy", true: "interrupted"}[interrupted], func(t *testing.T) {
					config := legacyConfig(t, sep)
					if interrupted {
						if err := os.Rename(filepath.Join(config.Root, "a"+sep+"b"), filepath.Join(config.Root, "a@b")); err != nil {
							t.Fatal(err)
						}
					}
					for i := 0; i < 2; i++ {
						l, err := New(config)
						if err != nil {
							t.Fatal(err)
						}
						sources := l.Sources()
						if len(sources) != 1 || sources[0].Repo != "a/b" || !sources[0].Skills[0].Enabled {
							t.Fatalf("sources = %+v", sources)
						}
						if err := l.Close(); err != nil {
							t.Fatal(err)
						}
					}
					link := filepath.Join(config.EnabledDir, "vam@a@b@tdd")
					if data, err := os.ReadFile(filepath.Join(link, "SKILL.md")); err != nil || string(data) != "original" {
						t.Fatalf("linked data = %q, %v", data, err)
					}
					if _, err := os.Lstat(filepath.Join(config.EnabledDir, "vam"+sep+"a"+sep+"b"+sep+"tdd")); !os.IsNotExist(err) {
						t.Fatal("legacy link remains")
					}
				})
			}
		})
	}
}

func TestMigrationConflictsPreserveOldDataAndLinks(t *testing.T) {
	for _, sep := range []string{"_@_", "___"} {
		t.Run(sep, func(t *testing.T) {
			for _, kind := range []string{"source", "file", "foreign-link"} {
				t.Run(kind, func(t *testing.T) {
					config := legacyConfig(t, sep)
					conflict := filepath.Join(config.EnabledDir, "vam@a@b@tdd")
					var err error
					switch kind {
					case "source":
						conflict = filepath.Join(config.Root, "a@b")
						err = os.Mkdir(conflict, 0o700)
					case "file":
						err = os.WriteFile(conflict, []byte("mine"), 0o600)
					case "foreign-link":
						err = os.Symlink(config.Tmp, conflict)
					}
					if err != nil {
						t.Fatal(err)
					}
					before, err := os.Lstat(conflict)
					if err != nil {
						t.Fatal(err)
					}
					if l, err := New(config); err == nil {
						l.Close()
						t.Fatal("conflict accepted")
					}
					after, err := os.Lstat(conflict)
					if err != nil || !os.SameFile(before, after) {
						t.Fatal("conflict changed")
					}
					legacy := filepath.Join(config.EnabledDir, "vam"+sep+"a"+sep+"b"+sep+"tdd", "SKILL.md")
					if data, err := os.ReadFile(legacy); err != nil || string(data) != "original" {
						t.Fatalf("legacy link broken: %q, %v", data, err)
					}
				})
			}
		})
	}
}
