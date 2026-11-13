// Package library is the skill library: the folder installed skills live in, and
// the SQLite index derived from it. The folder is the truth; the index is
// rebuilt from it, so it can never disagree with what is on disk.
package library

import (
	"database/sql"
	"fmt"
	"github.com/vivaui/viva/app"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "modernc.org/sqlite" // registers the "sqlite" driver

	"github.com/lizhian/viva-agents-manager/internal/domain"
)

// Config is where a library keeps its files, and who fetches repositories.
type Config struct {
	// Root is the folder installed skills live in.
	Root string
	// EnabledDir holds links exposed to agents; empty disables this capability.
	EnabledDir string
	// Tmp is where a repository is fetched before it is migrated into Root.
	Tmp string
	// DBPath is the SQLite file holding the index.
	DBPath string
	// Installer fetches a repository's skills.
	Installer domain.Installer
}

// Library is the skill library on this machine.
type Library struct {
	config Config
	// db is the index, nil when it could not be opened. The library lists and
	// installs without it; only the read path gets slower.
	db *sql.DB
	// mu serializes installs, updates and link changes.
	mu sync.Mutex
}

// Dir is where the application keeps everything on this machine.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".viva-agents-manager"), nil
}

// Open prepares the library under the application's own folder.
func Open(installer domain.Installer) (*Library, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return New(Config{
		Root:       filepath.Join(dir, "skills"),
		EnabledDir: filepath.Join(home, ".agents", "skills"),
		Tmp:        filepath.Join(dir, ".tmp"),
		DBPath:     filepath.Join(dir, "sqlite.db"),
		Installer:  installer,
	})
}

// New prepares a library at the given places: it makes the folders, clears what
// a previous run left in the temporary folder, and rebuilds the index from the
// disk.
func New(config Config) (*Library, error) {
	if config.Installer == nil {
		return nil, fmt.Errorf("library: no installer")
	}
	var err error
	config.Root, err = filepath.Abs(config.Root)
	if err != nil {
		return nil, err
	}
	if config.EnabledDir != "" {
		config.EnabledDir, err = filepath.Abs(config.EnabledDir)
		if err != nil {
			return nil, err
		}
	}
	for _, dir := range []string{config.Root, config.Tmp} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("library: %w", err)
		}
	}
	library := &Library{config: config}
	library.clearTmp()
	library.clearLeftovers()
	if err := library.migrateNames(); err != nil {
		return nil, err
	}
	db, err := openIndex(config.DBPath)
	if err != nil {
		// The index is derived, so losing it costs a slower listing, not data.
		log.Printf("library: 打开索引失败，改用磁盘扫描: %v", err)
		db = nil
	}
	library.db = db
	library.rebuild()
	return library, nil
}

// Close releases the index.
func (l *Library) Close() error {
	if l.db == nil {
		return nil
	}
	return l.db.Close()
}

// Sources lists the library. It reads the index, which the library keeps in step
// with the disk; without one it reads the disk itself.
func (l *Library) Sources() []domain.Source {
	if l.db != nil {
		sources, err := l.indexed()
		if err == nil {
			return l.withEnabled(sources)
		}
		log.Printf("library: 读取索引失败，改用磁盘扫描: %v", err)
	}
	return l.withEnabled(l.scan())
}

// OpenFolder locates the enabled skills directory in the system file manager.
func (l *Library) OpenFolder() error {
	if l.config.EnabledDir == "" {
		return domain.ErrUnsupported
	}
	if err := os.MkdirAll(l.config.EnabledDir, 0o755); err != nil {
		return err
	}
	return app.RevealPath(l.config.EnabledDir)
}

// clearTmp removes what a previous run left behind: a repository that was half
// fetched when the application quit has no use now.
func (l *Library) clearTmp() {
	entries, err := os.ReadDir(l.config.Tmp)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(l.config.Tmp, entry.Name())); err != nil {
			log.Printf("library: 清理临时目录失败: %v", err)
		}
	}
}

// clearLeftovers removes what an install or an update was holding when the
// application quit: a staging folder, the copy it was building, and the previous
// version a swap had moved aside. The scan ignores them, so without this they
// would sit in the library for good.
func (l *Library) clearLeftovers() {
	entries, err := os.ReadDir(l.config.Root)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, ".staging-") && !strings.HasPrefix(name, ".update-") &&
			!strings.HasPrefix(name, ".previous-") {
			continue
		}
		if err := os.RemoveAll(filepath.Join(l.config.Root, name)); err != nil {
			log.Printf("library: 清理 %s 失败: %v", name, err)
		}
	}
}

// OpenSkillFolder locates the installed directory, independent of enabled links.
func (l *Library) OpenSkillFolder(source, skill string) error {
	if !domain.ValidSourceName(source) || !domain.ValidSkillName(skill) {
		return domain.ErrNoDocument
	}
	dir, err := filepath.EvalSymlinks(filepath.Join(l.config.Root, source, skill))
	if err != nil {
		return err
	}
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("技能路径不是目录: %s", dir)
	}
	return app.RevealPath(dir)
}
