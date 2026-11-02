// Package skillinstall fetches a repository's skills with the skills npm package,
// run through npx. It deliberately does not reimplement how a repository lays its
// skills out: the CLI already knows, and it keeps knowing as repositories change.
package skillinstall

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lizhian/viva-agents-manager/internal/domain"
)

// pathProbeTimeout bounds the login-shell probe. A shell whose startup blocks
// would otherwise hang the install before npx is ever reached.
const pathProbeTimeout = 5 * time.Second

// Npx installs a repository's skills by running the skills CLI.
type Npx struct{}

// Prepare runs npx skills in dir and reports where each skill landed.
//
// The CLI installs into the project it is run from, so dir is both the working
// directory and the only place the results are read back from: anything it
// reports outside dir is refused.
func (Npx) Prepare(ctx context.Context, repo, dir string) ([]domain.PreparedSkill, error) {
	path := loginPath(ctx)
	npx, err := findNpx(path)
	if err != nil {
		return nil, err
	}
	run := func(args ...string) ([]byte, error) {
		command := exec.CommandContext(ctx, npx, args...)
		command.Dir = dir
		command.Env = append(os.Environ(), "PATH="+path)
		var stderr bytes.Buffer
		command.Stderr = &stderr
		out, err := command.Output()
		if err != nil {
			if message := strings.TrimSpace(stderr.String()); message != "" {
				return nil, fmt.Errorf("npx skills 失败: %s", lastLine(message))
			}
			return nil, fmt.Errorf("运行 npx skills 失败: %w", err)
		}
		return out, nil
	}
	if _, err := run("--yes", "skills", "add", repo, "-y"); err != nil {
		return nil, err
	}
	out, err := run("--yes", "skills", "list", "--json")
	if err != nil {
		return nil, err
	}
	return parse(dir, out)
}

// loginPath is the PATH to run npx with. An application started from the Finder
// inherits almost nothing, so a Node installed by mise, nvm or asdf would not be
// found; the login shell knows where it put it.
func loginPath(ctx context.Context) string {
	path := os.Getenv("PATH")
	shell := os.Getenv("SHELL")
	if !filepath.IsAbs(shell) {
		return path
	}
	probe, cancel := context.WithTimeout(ctx, pathProbeTimeout)
	defer cancel()
	command := exec.CommandContext(probe, shell, "-ilc", `printf '__VIVA_PATH__%s__VIVA_PATH__' "$PATH"`)
	out, err := command.Output()
	if err != nil {
		return path
	}
	// An interactive shell may print its own output, so the marker is searched
	// for rather than the whole output being trusted.
	text := string(out)
	start := strings.LastIndex(text, "__VIVA_PATH__")
	if start < 0 {
		return path
	}
	text = text[start+len("__VIVA_PATH__"):]
	end := strings.Index(text, "__VIVA_PATH__")
	if end < 0 {
		return path
	}
	found := text[:end]
	if found == "" {
		return path
	}
	return found + string(os.PathListSeparator) + path
}

// findNpx looks for an executable npx on the path.
func findNpx(path string) (string, error) {
	for _, folder := range filepath.SplitList(path) {
		candidate := filepath.Join(folder, "npx")
		info, err := os.Stat(candidate)
		if err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
			return candidate, nil
		}
	}
	return "", domain.ErrNpxMissing
}

// listed is the shape of one entry of `skills list --json`.
type listed struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// parse reads the JSON the CLI printed and checks every skill it names is inside
// the folder the CLI was run in.
func parse(dir string, out []byte) ([]domain.PreparedSkill, error) {
	start := bytes.IndexByte(out, '[')
	if start < 0 {
		return nil, errors.New("解析 npx skills 的输出失败")
	}
	var entries []listed
	if err := json.Unmarshal(out[start:], &entries); err != nil {
		return nil, fmt.Errorf("解析 npx skills 的输出失败: %w", err)
	}
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	prepared := make([]domain.PreparedSkill, 0, len(entries))
	for _, entry := range entries {
		name, path := strings.TrimSpace(entry.Name), strings.TrimSpace(entry.Path)
		if name == "" || path == "" {
			continue
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || !within(root, resolved) {
			return nil, fmt.Errorf("技能 %s 不在临时目录内", name)
		}
		prepared = append(prepared, domain.PreparedSkill{Name: name, Path: path})
	}
	return prepared, nil
}

// within reports whether path is root or sits inside it.
func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// lastLine is the most informative part of a CLI error: the CLI draws its own
// boxes, and the last line inside them is the sentence that says what broke.
func lastLine(message string) string {
	lines := strings.Split(message, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(strings.Trim(lines[i], "│╭╮╰╯─┌┐└┘├┤")); line != "" {
			return line
		}
	}
	return message
}
