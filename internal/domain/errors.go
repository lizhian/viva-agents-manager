package domain

import "errors"

// The ways an install can fail before the network is even reached, plus the two
// the desktop cannot do. The interface turns these into sentences; the raw
// messages are for logs and tests.
var (
	// ErrEmptyRepo is what Install returns when it is given nothing to install.
	ErrEmptyRepo = errors.New("domain: empty repository")
	// ErrBadRepo is what Install returns for something that is not "owner/repo".
	ErrBadRepo = errors.New("domain: repository is not owner/repo")
	// ErrAlreadyInstalled is what Install returns when the library already holds
	// that repository.
	ErrAlreadyInstalled = errors.New("domain: repository already installed")
	// ErrNoSkills is what Install returns when a repository holds no skill.
	ErrNoSkills = errors.New("domain: repository has no installable skills")
	// ErrNpxMissing is what an installer returns when no npx is on the path.
	ErrNpxMissing = errors.New("domain: npx not found")
	// ErrUnsupported is what a Store returns for a desktop capability the
	// framework does not offer on this platform.
	ErrUnsupported = errors.New("domain: not supported on this platform")
)
