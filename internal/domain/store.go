package domain

import "context"

// Progress is how far an install has got, in the order the stages happen. The
// interface decides what each stage is called on screen.
type Progress int

const (
	// ProgressPreparing is the repository being fetched into the temporary folder.
	ProgressPreparing Progress = iota
	// ProgressMigrating is the fetched skills being written into the library.
	ProgressMigrating
	// ProgressDone is the source installed, migrated and indexed.
	ProgressDone
)

// UpdateProgress is how far updating the library has got.
type UpdateProgress struct {
	// Source is the repository being fetched.
	Source string
	// Done is how many sources are finished, Total how many there are.
	Done, Total int
	// Stage is what the source in flight is doing.
	Stage Progress
}

// UpdateResult is what updating one source did. A source that could not be
// updated keeps what it had.
type UpdateResult struct {
	Source string
	Err    error
}

// Store is what the application needs from wherever skills live: the local
// library, and the repositories it was installed from. The interface exists so
// the interface never learns how a source is fetched.
type Store interface {
	// Sources lists the sources in the order the tree shows them.
	Sources() []Source
	// Install fetches a repository named "owner/repo" into the library, reporting
	// each stage through progress, which may be nil. It blocks for as long as the
	// network does, so the caller runs it off the window thread and cancels it
	// through ctx.
	Install(ctx context.Context, repo string, progress func(Progress)) error
	// UpdateAll fetches every source that has a repository again and replaces what
	// the library holds with it, so the two match: a skill the repository no
	// longer has is gone afterwards. A source that fails keeps what it had and the
	// rest still update. It blocks, so the caller runs it off the window thread and
	// cancels it through ctx.
	UpdateAll(ctx context.Context, progress func(UpdateProgress)) []UpdateResult
	// Document reads one file of a skill's directory: what it declares about
	// itself, and its body. The skill's own document is "SKILL.md".
	Document(source, skill, file string) (Document, error)
	// SetEnabled enables one skill, or all skills of a source when skill is empty.
	// Sources reports the actual state after partial failures.
	SetEnabled(source, skill string, enabled bool) error
	// SetExpired marks a skill as expired and disables it; clearing the mark
	// does not enable it. Marks survive source updates.
	SetExpired(source, skill string, expired bool) error
	// OpenFolder asks the desktop to show where skills live.
	OpenFolder() error
	// OpenSkillFolder shows the actual directory of one installed skill.
	OpenSkillFolder(source, skill string) error
}

// PreparedSkill is one skill an installer left in the folder it was given.
type PreparedSkill struct {
	// Name is the skill's own name, which is also the directory it is installed as.
	Name string
	// Path is where the installer put it.
	Path string
}

// Installer fetches a repository's skills into a directory it is given and
// reports where each one landed. It is an interface so installing can be tested
// without running npx.
type Installer interface {
	Prepare(ctx context.Context, repo, dir string) ([]PreparedSkill, error)
}
