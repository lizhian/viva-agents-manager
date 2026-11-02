package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lizhian/viva-agents-manager/internal/domain"
)

// fakeSkill is one skill a fake installer writes.
type fakeSkill struct {
	Name  string
	Files map[string]string
}

// fakeInstaller writes the skills it was given into the folder it is handed, the
// way the skills CLI would, and reports where each one landed.
type fakeInstaller struct {
	skills []fakeSkill
	err    error
}

func (f fakeInstaller) Prepare(ctx context.Context, repo, dir string) ([]domain.PreparedSkill, error) {
	if f.err != nil {
		return nil, f.err
	}
	prepared := make([]domain.PreparedSkill, 0, len(f.skills))
	for _, skill := range f.skills {
		root := filepath.Join(dir, ".agents", "skills", skill.Name)
		for path, content := range skill.Files {
			full := filepath.Join(root, path)
			if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
				return nil, err
			}
			if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
				return nil, err
			}
		}
		prepared = append(prepared, domain.PreparedSkill{Name: skill.Name, Path: root})
	}
	return prepared, nil
}

// twoSkills is what most of these tests install.
func twoSkills() fakeInstaller {
	return fakeInstaller{skills: []fakeSkill{
		{Name: "tdd", Files: map[string]string{"SKILL.md": "# tdd", "notes.md": "notes"}},
		{Name: "code-review", Files: map[string]string{"SKILL.md": "# review"}},
	}}
}

// newLibrary builds a library over a throwaway folder.
func newLibrary(t *testing.T, installer domain.Installer) *Library {
	t.Helper()
	dir := t.TempDir()
	library, err := New(Config{
		Root:      filepath.Join(dir, "skills"),
		Tmp:       filepath.Join(dir, ".tmp"),
		DBPath:    filepath.Join(dir, "sqlite.db"),
		Installer: installer,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { library.Close() })
	return library
}

// names lists the skill names a source holds, in tree order.
func names(source domain.Source) []string {
	names := make([]string, len(source.Skills))
	for i, skill := range source.Skills {
		names[i] = skill.Name
	}
	return names
}

// TestInstallMigratesIntoTheLibrary checks the whole path: a repository is
// fetched into the temporary folder, migrated under its safe name, and listed
// with its skills.
func TestInstallMigratesIntoTheLibrary(t *testing.T) {
	library := newLibrary(t, twoSkills())

	var stages []domain.Progress
	if err := library.Install(context.Background(), "mattpocock/skills", func(progress domain.Progress) {
		stages = append(stages, progress)
	}); err != nil {
		t.Fatal(err)
	}

	source := filepath.Join(library.config.Root, "mattpocock_@_skills")
	for _, skill := range []string{"tdd", "code-review"} {
		if _, err := os.Stat(filepath.Join(source, skill, "SKILL.md")); err != nil {
			t.Fatalf("%s was not migrated: %v", skill, err)
		}
	}
	if _, err := os.Stat(filepath.Join(source, "tdd", "notes.md")); err != nil {
		t.Fatalf("a skill's other files were not copied: %v", err)
	}

	sources := library.Sources()
	if len(sources) != 1 {
		t.Fatalf("source count = %d, want 1", len(sources))
	}
	if sources[0].Repo != "mattpocock/skills" || sources[0].Label() != "mattpocock/skills" {
		t.Fatalf("source = %+v, want the repository it came from", sources[0])
	}
	if got := names(sources[0]); len(got) != 2 || got[0] != "code-review" || got[1] != "tdd" {
		t.Fatalf("skills = %v, want both, in tree order", got)
	}
	if len(stages) == 0 || stages[len(stages)-1] != domain.ProgressDone {
		t.Fatalf("stages = %v, want the last one to be done", stages)
	}
}

// TestInstallLeavesTheTemporaryFolderEmpty checks that a finished install takes
// its fetched copy with it.
func TestInstallLeavesTheTemporaryFolderEmpty(t *testing.T) {
	library := newLibrary(t, twoSkills())
	if err := library.Install(context.Background(), "a/b", nil); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(library.config.Tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("the temporary folder still holds %d entries", len(entries))
	}
}

// TestInstallRefusesASecondTime checks that the same repository cannot be
// installed twice, and that the first install is untouched.
func TestInstallRefusesASecondTime(t *testing.T) {
	library := newLibrary(t, twoSkills())
	if err := library.Install(context.Background(), "a/b", nil); err != nil {
		t.Fatal(err)
	}
	if err := library.Install(context.Background(), "a/b", nil); !errors.Is(err, domain.ErrAlreadyInstalled) {
		t.Fatalf("second install = %v, want ErrAlreadyInstalled", err)
	}
	if sources := library.Sources(); len(sources) != 1 {
		t.Fatalf("source count after the refused install = %d, want 1", len(sources))
	}
}

// TestInstallRejectsBadRepositories checks what never reaches the installer.
func TestInstallRejectsBadRepositories(t *testing.T) {
	library := newLibrary(t, twoSkills())
	if err := library.Install(context.Background(), "  ", nil); !errors.Is(err, domain.ErrEmptyRepo) {
		t.Fatalf("empty repository = %v, want ErrEmptyRepo", err)
	}
	if err := library.Install(context.Background(), "a/b/c", nil); !errors.Is(err, domain.ErrBadRepo) {
		t.Fatalf("path = %v, want ErrBadRepo", err)
	}
}

// TestInstallFailureLeavesNothing checks that a repository the installer could
// not fetch writes neither a mirror nor an index row.
func TestInstallFailureLeavesNothing(t *testing.T) {
	failure := errors.New("no network")
	library := newLibrary(t, fakeInstaller{err: failure})

	if err := library.Install(context.Background(), "a/b", nil); !errors.Is(err, failure) {
		t.Fatalf("install = %v, want the installer's error", err)
	}
	if _, err := os.Stat(filepath.Join(library.config.Root, "a_@_b")); !os.IsNotExist(err) {
		t.Fatal("a failed install left a mirror behind")
	}
	if sources := library.Sources(); len(sources) != 0 {
		t.Fatalf("a failed install left %d sources", len(sources))
	}
}

// TestInstallRefusesAnEmptyRepository checks that a repository holding no skill
// is reported rather than installed as an empty source.
func TestInstallRefusesAnEmptyRepository(t *testing.T) {
	library := newLibrary(t, fakeInstaller{})
	if err := library.Install(context.Background(), "a/b", nil); !errors.Is(err, domain.ErrNoSkills) {
		t.Fatalf("install = %v, want ErrNoSkills", err)
	}
}

// TestInstallRefusesBadSkillNames checks that a skill cannot name a directory
// outside the source it belongs to.
func TestInstallRefusesBadSkillNames(t *testing.T) {
	for _, name := range []string{"../escape", "a/b"} {
		library := newLibrary(t, fakeInstaller{skills: []fakeSkill{
			{Name: name, Files: map[string]string{"SKILL.md": "# x"}},
		}})
		if err := library.Install(context.Background(), "a/b", nil); err == nil {
			t.Fatalf("installing a skill named %q was accepted", name)
		}
		if _, err := os.Stat(filepath.Join(library.config.Root, "a_@_b")); !os.IsNotExist(err) {
			t.Fatalf("skill %q left a mirror behind", name)
		}
	}
}

// TestInstallRequiresTheSkillDocument checks that a directory without a SKILL.md
// is not treated as a skill.
func TestInstallRequiresTheSkillDocument(t *testing.T) {
	library := newLibrary(t, fakeInstaller{skills: []fakeSkill{
		{Name: "empty", Files: map[string]string{"notes.md": "notes"}},
	}})
	if err := library.Install(context.Background(), "a/b", nil); err == nil {
		t.Fatal("a skill without a SKILL.md was installed")
	}
}

// TestScanFindsLocalFolders checks that a folder someone put in the library by
// hand is a source of its own, with no repository.
func TestScanFindsLocalFolders(t *testing.T) {
	library := newLibrary(t, twoSkills())
	mine := filepath.Join(library.config.Root, "我的技能", "notes")
	if err := os.MkdirAll(mine, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mine, "SKILL.md"), []byte("# notes"), 0o600); err != nil {
		t.Fatal(err)
	}
	library.rebuild()

	sources := library.Sources()
	if len(sources) != 1 {
		t.Fatalf("source count = %d, want 1", len(sources))
	}
	if sources[0].Repo != "" || sources[0].Label() != "我的技能" {
		t.Fatalf("local source = %+v, want no repository and its own name", sources[0])
	}
}

// TestScanSkipsFoldersWithNoSkill checks that the staging folder an install
// leaves for a moment, and a folder holding nothing, are not sources.
func TestScanSkipsFoldersWithNoSkill(t *testing.T) {
	library := newLibrary(t, twoSkills())
	for _, name := range []string{".staging-123", "empty"} {
		if err := os.MkdirAll(filepath.Join(library.config.Root, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	library.rebuild()
	if sources := library.Sources(); len(sources) != 0 {
		t.Fatalf("source count = %d, want none", len(sources))
	}
}

// TestIndexFollowsTheDisk checks the promise the index makes: opening the library
// again rebuilds it, so a folder deleted by hand stops being listed.
func TestIndexFollowsTheDisk(t *testing.T) {
	dir := t.TempDir()
	config := Config{
		Root:      filepath.Join(dir, "skills"),
		Tmp:       filepath.Join(dir, ".tmp"),
		DBPath:    filepath.Join(dir, "sqlite.db"),
		Installer: twoSkills(),
	}
	first, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Install(context.Background(), "a/b", nil); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { second.Close() })
	if sources := second.Sources(); len(sources) != 1 {
		t.Fatalf("source count = %d, want 1 after reopening", len(sources))
	}

	if err := os.RemoveAll(filepath.Join(config.Root, "a_@_b")); err != nil {
		t.Fatal(err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	third, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { third.Close() })
	if sources := third.Sources(); len(sources) != 0 {
		t.Fatalf("a folder deleted by hand is still listed: %d sources", len(sources))
	}
}

// TestNewClearsTheTemporaryFolder checks that what a previous run left half
// fetched is gone by the time the library is ready.
func TestNewClearsTheTemporaryFolder(t *testing.T) {
	dir := t.TempDir()
	tmp := filepath.Join(dir, ".tmp")
	if err := os.MkdirAll(filepath.Join(tmp, "install-123"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "install-123", "leftover"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	library, err := New(Config{
		Root:      filepath.Join(dir, "skills"),
		Tmp:       tmp,
		DBPath:    filepath.Join(dir, "sqlite.db"),
		Installer: twoSkills(),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { library.Close() })

	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("the temporary folder still holds %d entries", len(entries))
	}
}

// TestNewClearsStaging checks that a migration interrupted between its copy and
// its rename does not leave a folder in the library for good.
func TestNewClearsStaging(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "skills")
	staging := filepath.Join(root, ".staging-123")
	if err := os.MkdirAll(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	library, err := New(Config{
		Root:      root,
		Tmp:       filepath.Join(dir, ".tmp"),
		DBPath:    filepath.Join(dir, "sqlite.db"),
		Installer: twoSkills(),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { library.Close() })

	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatal("an interrupted migration left its staging folder behind")
	}
}

// TestCopyTreeRefusesAnEscapingLink checks that a repository cannot make the
// copy write or read outside the skill it shipped.
func TestCopyTreeRefusesAnEscapingLink(t *testing.T) {
	dir := t.TempDir()
	skill := filepath.Join(dir, "skill")
	if err := os.MkdirAll(skill, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("# x"), 0o600); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(dir, "secret")
	if err := os.WriteFile(secret, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(skill, "leak")); err != nil {
		t.Fatal(err)
	}
	if err := copyTree(skill, filepath.Join(dir, "out")); err == nil {
		t.Fatal("copyTree accepted a link that leaves the skill")
	}
}

// TestOpenFolderIsUnsupported pins the framework gap: Viva cannot reveal a
// directory, so the store says so rather than pretending.
func TestOpenFolderIsUnsupported(t *testing.T) {
	library := newLibrary(t, twoSkills())
	if err := library.OpenFolder(); !errors.Is(err, domain.ErrUnsupported) {
		t.Fatalf("OpenFolder = %v, want ErrUnsupported", err)
	}
}

// TestDocumentReadsTheSkillFile checks the whole read: the file on disk becomes
// what the detail column shows.
func TestDocumentReadsTheSkillFile(t *testing.T) {
	library := newLibrary(t, twoSkills())
	if err := library.Install(context.Background(), "a/b", nil); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(library.config.Root, "a_@_b", "tdd", "SKILL.md")
	declared := "---\nname: Declared\ndescription: From the file.\n---\n\n# Body\n"
	if err := os.WriteFile(path, []byte(declared), 0o600); err != nil {
		t.Fatal(err)
	}

	document, err := library.Document("a_@_b", "tdd")
	if err != nil {
		t.Fatal(err)
	}
	if document.Name != "Declared" || document.Description != "From the file." || document.Body != "# Body\n" {
		t.Fatalf("document = %+v, want what the file declares", document)
	}
}

// TestDocumentRejectsNamesThatArePaths checks the one place two names are joined
// into a path: neither may walk out of the library.
func TestDocumentRejectsNamesThatArePaths(t *testing.T) {
	library := newLibrary(t, twoSkills())
	for _, names := range [][2]string{
		{"..", "tdd"},
		{"a_@_b", "../tdd"},
		{"a/b", "tdd"},
		{"", "tdd"},
		{"a_@_b", ""},
	} {
		if _, err := library.Document(names[0], names[1]); !errors.Is(err, domain.ErrNoDocument) {
			t.Fatalf("Document(%q, %q) = %v, want ErrNoDocument", names[0], names[1], err)
		}
	}
}

// TestDocumentReportsAMissingFile checks that a skill whose file is gone says so
// rather than showing an empty page.
func TestDocumentReportsAMissingFile(t *testing.T) {
	library := newLibrary(t, twoSkills())
	if _, err := library.Document("a_@_b", "tdd"); err == nil {
		t.Fatal("a missing document was read")
	}
}

// TestDocumentRefusesAHugeFile checks that one oversized file cannot stall the
// window thread.
func TestDocumentRefusesAHugeFile(t *testing.T) {
	library := newLibrary(t, twoSkills())
	if err := library.Install(context.Background(), "a/b", nil); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(library.config.Root, "a_@_b", "tdd", "SKILL.md")
	if err := os.Truncate(path, maxDocumentSize+1); err != nil {
		t.Fatal(err)
	}
	_, err := library.Document("a_@_b", "tdd")
	if err == nil || !strings.Contains(err.Error(), "MiB") {
		t.Fatalf("Document of an oversized file = %v, want a size complaint", err)
	}
}
