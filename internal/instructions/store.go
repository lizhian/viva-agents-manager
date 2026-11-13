package instructions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"
)

var ErrConflict = errors.New("AGENTS.md 已被外部修改，生成已暂停")

// Observation binds a resolution choice to the exact external version shown.
type Observation struct {
	Digest  string
	Body    []byte
	Missing bool
}
type Config struct{ Root, Target string }
type Store struct {
	Config Config
	Data   Snapshot
	loaded bool
}
type View struct {
	Hidden      bool    `json:"hidden,omitempty"`
	EditorSplit float32 `json:"editor_split,omitempty"`
	Mode        string  `json:"mode"`
	Selected    string  `json:"selected"`
	Width       float32 `json:"width"`
}

func DefaultConfig() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, err
	}
	return Config{Root: filepath.Join(home, ".viva-agents-manager", "agentsmd"), Target: filepath.Join(home, ".agents", "AGENTS.md")}, nil
}

// Open only reads. First-use initialization is explicit, so navigation and tests
// never claim a user's target merely by constructing a window.
func Open(config Config) (*Store, error) {
	s := &Store{Config: config, Data: Snapshot{Version: 2}}
	b, err := os.ReadFile(s.path())
	if errors.Is(err, os.ErrNotExist) {
		s.loaded = true
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err = json.Unmarshal(b, &s.Data); err != nil {
		return s, fmt.Errorf("读取指令库: %w", err)
	}
	if s.Data.Version == 1 {
		if err = migrateV1(b, &s.Data); err != nil {
			return s, err
		}
	}
	if err = s.Data.Validate(); err != nil {
		return s, err
	}
	s.loaded = true
	return s, nil
}
func (s *Store) path() string { return filepath.Join(s.Config.Root, "library.json") }
func (s *Store) Save() error {
	if !s.loaded {
		return errors.New("指令库未成功读取，禁止覆盖")
	}
	if err := s.Data.Validate(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.Data, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(s.path(), b)
}
func digest(b []byte) string { v := sha256.Sum256(b); return hex.EncodeToString(v[:]) }
func (s *Store) Observe() (Observation, error) {
	b, err := os.ReadFile(s.Config.Target)
	if errors.Is(err, os.ErrNotExist) {
		return Observation{Digest: "missing", Missing: true}, nil
	}
	if err != nil {
		return Observation{}, err
	}
	return Observation{Digest: digest(b), Body: b}, nil
}
func (s *Store) Initialize() error {
	if !s.loaded {
		return errors.New("指令库读取失败")
	}
	if !s.Data.Initialized {
		o, err := s.Observe()
		if err != nil {
			return err
		}
		if len(o.Body) > 0 {
			if !utf8.Valid(o.Body) {
				return errors.New("已有 AGENTS.md 必须为 UTF-8，未接管")
			}
			id := s.Data.Add("原有指令", string(o.Body))
			s.Data.Snippet(id).Enabled = true
		}
		s.Data.Initialized = true
		s.Data.Baseline = o.Digest
		if err = s.Save(); err != nil {
			s.Data = Snapshot{Version: 2}
			return err
		}
	}
	return s.Generate()
}
func (s *Store) conflict() error {
	s.Data.Paused = true
	if err := s.Save(); err != nil {
		return errors.Join(ErrConflict, err)
	}
	return ErrConflict
}
func (s *Store) Generate() error {
	if !s.Data.Initialized {
		return errors.New("尚未接管输出文件")
	}
	if err := s.Save(); err != nil {
		return err
	}
	current, err := s.Observe()
	if err != nil {
		return err
	}
	if s.Data.Paused {
		return ErrConflict
	}
	// Pending is durable before output. Either digest is recoverable after a crash.
	if s.Data.Pending != "" && current.Digest == s.Data.Pending {
		s.Data.Baseline = current.Digest
		s.Data.Pending = ""
		if err = s.Save(); err != nil {
			return err
		}
	}
	if current.Digest != s.Data.Baseline {
		return s.conflict()
	}
	body := []byte(s.Data.Merge())
	next := digest(body)
	if current.Digest == next {
		s.Data.Pending = ""
		return s.Save()
	}
	s.Data.Pending = next
	if err = s.Save(); err != nil {
		return err
	}
	check, err := s.Observe()
	if err != nil {
		return err
	}
	if check.Digest != current.Digest {
		return s.conflict()
	}
	if err = atomicWrite(s.Config.Target, body); err != nil {
		return err
	}
	s.Data.Baseline = next
	s.Data.Pending = ""
	return s.Save()
}
func (s *Store) Resolve(observed Observation, keep bool) error {
	now, err := s.Observe()
	if err != nil {
		return err
	}
	if now.Digest != observed.Digest {
		return s.conflict()
	}
	if keep && !now.Missing {
		if !utf8.Valid(now.Body) {
			return errors.New("外部 AGENTS.md 必须为 UTF-8，未导入")
		}
		s.Data.Add("外部修改的指令", string(now.Body))
	}
	s.Data.Baseline = now.Digest
	s.Data.Pending = ""
	s.Data.Paused = false
	if err = s.Save(); err != nil {
		s.Data.Paused = true
		return err
	}
	return s.Generate()
}
func (s *Store) LoadView() View {
	v := View{Mode: "source", Width: 244}
	b, err := os.ReadFile(filepath.Join(s.Config.Root, "view.json"))
	if err == nil {
		var saved View
		if json.Unmarshal(b, &saved) == nil {
			v = saved
		}
	}
	if v.Width < 180 {
		v.Width = 244
	}
	return v
}
func (s *Store) SaveView(v View) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return atomicWrite(filepath.Join(s.Config.Root, "view.json"), b)
}
func atomicWrite(path string, b []byte) (err error) {
	dir := filepath.Dir(path)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".agentsmd-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(name, path)
}

// UpToDate reports whether the durable generation baseline matches this library.
// It does not read the target: external edits are checked before writes and at startup.
func (s *Store) UpToDate() bool {
	return s.Data.Initialized && !s.Data.Paused && s.Data.Pending == "" && s.Data.Baseline == digest([]byte(s.Data.Merge()))
}
