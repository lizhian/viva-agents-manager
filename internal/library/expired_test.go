package library

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lizhian/viva-agents-manager/internal/domain"
)

func TestExpiredSkillStaysDisabledAcrossUpdatesAndReopening(t *testing.T) {
	l := newLibrary(t, twoSkills())
	if err := l.Install(context.Background(), "a/b", nil); err != nil {
		t.Fatal(err)
	}
	if err := l.SetEnabled("a@b", "tdd", true); err != nil {
		t.Fatal(err)
	}
	if err := l.SetExpired("a@b", "tdd", true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(l.linkPath("a@b", "tdd")); !os.IsNotExist(err) {
		t.Fatal("expiration left enabled link")
	}
	if err := l.SetEnabled("a@b", "tdd", true); !errors.Is(err, domain.ErrExpired) {
		t.Fatalf("enable expired = %v", err)
	}
	if err := l.SetEnabled("a@b", "", true); err != nil {
		t.Fatal(err)
	}
	skills := l.Sources()[0].Skills
	if !skills[0].Enabled || skills[1].Enabled || !skills[1].Expired {
		t.Fatalf("bulk state = %+v", skills)
	}
	results := l.UpdateAll(context.Background(), nil)
	if results[0].Err != nil {
		t.Fatal(results[0].Err)
	}
	reopened, err := New(l.config)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	skills = reopened.Sources()[0].Skills
	if !skills[1].Expired || skills[1].Enabled {
		t.Fatalf("reopened state = %+v", skills)
	}
	if err := reopened.SetExpired("a@b", "tdd", false); err != nil {
		t.Fatal(err)
	}
	if skills := reopened.Sources()[0].Skills; skills[1].Enabled || skills[1].Expired {
		t.Fatalf("clearing mark enabled the skill: %+v", skills)
	}
	if err := reopened.SetEnabled("a@b", "tdd", true); err != nil {
		t.Fatal(err)
	}
}

func TestExpirationConflictKeepsOriginalAndDoesNotMarkExpired(t *testing.T) {
	l := newLibrary(t, twoSkills())
	if err := l.Install(context.Background(), "a/b", nil); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(l.config.EnabledDir, 0o700); err != nil {
		t.Fatal(err)
	}
	link := l.linkPath("a@b", "tdd")
	if err := os.WriteFile(link, []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := l.SetExpired("a@b", "tdd", true); err == nil {
		t.Fatal("conflict not reported")
	}
	if data, err := os.ReadFile(link); err != nil || string(data) != "mine" {
		t.Fatal("original changed")
	}
	if l.Sources()[0].Skills[1].Expired {
		t.Fatal("failed expiration persisted a mark")
	}
}

func TestVersionOneDatabaseUpgradesWithoutLosingExpirationOnRebuild(t *testing.T) {
	dir := t.TempDir()
	config := Config{Root: filepath.Join(dir, "skills"), Tmp: filepath.Join(dir, "tmp"), DBPath: filepath.Join(dir, "db.sqlite"), EnabledDir: filepath.Join(dir, "enabled"), Installer: twoSkills()}
	db, err := sql.Open("sqlite", config.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrations[0](db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA user_version = 1`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	l, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if err := l.Install(context.Background(), "a/b", nil); err != nil {
		t.Fatal(err)
	}
	if err := l.SetExpired("a@b", "tdd", true); err != nil {
		t.Fatal(err)
	}
	l.rebuild()
	if !l.Sources()[0].Skills[1].Expired {
		t.Fatal("index rebuild lost expiration")
	}
}
