package domain

import "errors"

// The ways an install can fail before the network is even reached, plus the two
// the desktop cannot do. The interface turns these into sentences; the raw
// messages are for logs and tests.
var (
	ErrExpired = errors.New("技能已过期，请先取消过期标记")
	// ErrEmptyRepo is what Install returns when it is given nothing to install.
	ErrEmptyRepo = errors.New("domain: empty repository")
	// ErrBadRepo is what Install returns for something that is not "owner/repo".
	ErrBadRepo = errors.New("仓库须为 owner/repo，@ 为保留分隔符")
	// ErrAlreadyInstalled is what Install returns when the library already holds
	// that repository.
	ErrAlreadyInstalled = errors.New("domain: repository already installed")
	// ErrNoSkills is what Install returns when a repository holds no skill.
	ErrNoSkills = errors.New("domain: repository has no installable skills")
	// ErrNpxMissing is what an installer returns when no npx is on the path.
	ErrNpxMissing = errors.New("domain: npx not found")
	// ErrNoDocument is what Document returns when the skill is not in the store,
	// or when the name it was given could not be one.
	ErrNoDocument = errors.New("domain: no such skill document")
	// ErrUnsupported is what a Store returns for a desktop capability the
	// framework does not offer on this platform.
	ErrUnsupported = errors.New("domain: not supported on this platform")
)
