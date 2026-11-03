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
	// OpenFolder asks the desktop to show where skills live.
	OpenFolder() error
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
