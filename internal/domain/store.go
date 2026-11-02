package domain

// Store is what the application needs from wherever skills live: the local
// install, a GitHub repository, or both later. The interface exists so the
// interface never learns how a source is fetched.
type Store interface {
	// Sources lists the skill sources in the order the tree shows them.
	Sources() []Source
	// Install adds a source from a GitHub repository named "owner/repo".
	Install(repo string) error
	// UpdateAll refreshes every source that has a repository.
	UpdateAll() error
	// OpenFolder asks the desktop to show where skills live.
	OpenFolder() error
}
