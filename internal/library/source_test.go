package library

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/lizhian/viva-agents-manager/internal/domain"
)

type recordingInstaller struct {
	fakeInstaller
	addresses []string
}

func (r *recordingInstaller) Prepare(ctx context.Context, address, dir string) ([]domain.PreparedSkill, error) {
	r.addresses = append(r.addresses, address)
	return r.fakeInstaller.Prepare(ctx, address, dir)
}

func TestURLSourceLifecycle(t *testing.T) {
	for _, address := range []string{"https://archify.si", "https://archify.si/skills?q=1#one", "http://localhost:8080/skills"} {
		t.Run(address, func(t *testing.T) {
			ctx := context.Background()
			installer := &recordingInstaller{fakeInstaller: twoSkills()}
			lib := newLibrary(t, installer)
			if err := lib.Install(ctx, address, nil); err != nil {
				t.Fatal(err)
			}
			name := domain.SafeName(address)
			if err := lib.Install(ctx, address, nil); !errors.Is(err, domain.ErrAlreadyInstalled) {
				t.Fatalf("duplicate = %v", err)
			}
			if err := lib.SetEnabled(name, "tdd", true); err != nil {
				t.Fatal(err)
			}
			if err := lib.SetExpired(name, "code-review", true); err != nil {
				t.Fatal(err)
			}
			if _, err := lib.Document(name, "tdd", "SKILL.md"); err != nil {
				t.Fatal(err)
			}
			// Reopening rebuilds the index from directories and their address metadata.
			config := lib.config
			if err := lib.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := New(config)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			sources := reopened.Sources()
			if len(sources) != 1 || sources[0].Repo != address || sources[0].Label() != address {
				t.Fatalf("after restart: %+v", sources)
			}
			// Exercise the disk fallback too, independently of the index.
			if scanned := reopened.scan(); len(scanned) != 1 || scanned[0].Repo != address {
				t.Fatalf("disk scan: %+v", scanned)
			}
			installer.err = errors.New("offline")
			if results := reopened.UpdateAll(ctx, nil); len(results) != 1 || results[0].Err == nil {
				t.Fatalf("failed update: %+v", results)
			}
			if _, err := reopened.Document(name, "tdd", "SKILL.md"); err != nil {
				t.Fatal(err)
			}
			installer.err = nil
			installer.skills = []fakeSkill{{Name: "tdd", Files: map[string]string{"SKILL.md": "# updated"}}}
			if results := reopened.UpdateAll(ctx, nil); len(results) != 1 || results[0].Err != nil {
				t.Fatalf("update: %+v", results)
			}
			if installer.addresses[len(installer.addresses)-1] != address {
				t.Fatalf("update lost original address: %v", installer.addresses)
			}
			sources = reopened.Sources()
			if len(sources) != 1 || sources[0].Repo != address || len(sources[0].Skills) != 1 || !sources[0].Skills[0].Enabled {
				t.Fatalf("after update: %+v", sources)
			}
			if err := reopened.Remove(name); err != nil {
				t.Fatal(err)
			}
			if len(reopened.Sources()) != 0 {
				t.Fatal("removed source remains indexed")
			}
			for _, path := range []string{filepath.Join(config.Root, name), reopened.linkPath(name, "tdd")} {
				if _, err := os.Lstat(path); !os.IsNotExist(err) {
					t.Fatalf("removed path remains: %s, %v", path, err)
				}
			}
			// Removal must permit a fresh install under the same identity.
			if err := reopened.Install(ctx, address, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGitHubURLDuplicate(t *testing.T) {
	lib := newLibrary(t, twoSkills())
	if err := lib.Install(context.Background(), "a/b", nil); err != nil {
		t.Fatal(err)
	}
	if err := lib.Install(context.Background(), "https://github.com/a/b", nil); !errors.Is(err, domain.ErrAlreadyInstalled) {
		t.Fatalf("duplicate = %v", err)
	}
}

func TestURLPrefixDoesNotHideRepository(t *testing.T) {
	lib := newLibrary(t, twoSkills())
	if err := lib.Install(context.Background(), "url-owner/skills", nil); err != nil {
		t.Fatal(err)
	}
	sources := lib.Sources()
	if len(sources) != 1 || sources[0].Repo != "url-owner/skills" {
		t.Fatalf("existing repository lost identity: %+v", sources)
	}
}

func TestLegacyURLNamesStayInstalled(t *testing.T) {
	for _, address := range []string{"https://archify.si", "https://archify.si/"} {
		t.Run(address, func(t *testing.T) {
			ctx := context.Background()
			lib := newLibrary(t, twoSkills())
			if err := lib.Install(ctx, address, nil); err != nil {
				t.Fatal(err)
			}
			// Reproduce the former on-disk format without relying on the new namer.
			legacy := fmt.Sprintf("url-%x", sha256.Sum256([]byte(address)))
			if err := os.Rename(filepath.Join(lib.config.Root, "archify.si"), filepath.Join(lib.config.Root, legacy)); err != nil {
				t.Fatal(err)
			}
			lib.rebuild()
			if err := lib.SetEnabled(legacy, "tdd", true); err != nil {
				t.Fatal(err)
			}
			if err := lib.SetExpired(legacy, "code-review", true); err != nil {
				t.Fatal(err)
			}
			config := lib.config
			lib.Close()
			reopened, err := New(config)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			for _, variant := range []string{"https://archify.si", "https://archify.si/"} {
				if err := reopened.Install(ctx, variant, nil); !errors.Is(err, domain.ErrAlreadyInstalled) {
					t.Fatalf("legacy duplicate = %v", err)
				}
			}
			if results := reopened.UpdateAll(ctx, nil); len(results) != 1 || results[0].Err != nil {
				t.Fatalf("legacy update = %+v", results)
			}
			sources := reopened.Sources()
			if len(sources) != 1 || sources[0].Name != legacy || sources[0].Repo != address {
				t.Fatalf("legacy source changed: %+v", sources)
			}
			for _, skill := range sources[0].Skills {
				if skill.Name == "tdd" && !skill.Enabled {
					t.Fatal("legacy enabled link lost")
				}
				if skill.Name == "code-review" && !skill.Expired {
					t.Fatal("legacy expiration lost")
				}
			}
			if _, err := os.Stat(filepath.Join(config.Root, "archify.si")); !os.IsNotExist(err) {
				t.Fatalf("unexpected migration: %v", err)
			}
		})
	}
}

func TestReadableURLNameConflictPreservesLocalSource(t *testing.T) {
	lib := newLibrary(t, twoSkills())
	local := filepath.Join(lib.config.Root, "archify.si")
	if err := os.MkdirAll(local, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(local, "keep.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := lib.Install(context.Background(), "https://archify.si", nil); err == nil || errors.Is(err, domain.ErrAlreadyInstalled) {
		t.Fatalf("local conflict = %v", err)
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "keep" {
		t.Fatalf("local data changed: %q, %v", data, err)
	}
}
