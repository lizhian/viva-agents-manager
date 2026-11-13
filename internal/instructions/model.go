// Package instructions owns the local instruction library and its generated AGENTS.md.
package instructions

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"time"
)

type Snippet struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Body    string    `json:"body"`
	Tags    []string  `json:"tags,omitempty"`
	Order   int       `json:"order"`
	Enabled bool      `json:"enabled"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}
type Entry struct {
	ID, Name string
	Enabled  bool
}
type Snapshot struct {
	Version     int       `json:"version"`
	Snippets    []Snippet `json:"snippets"`
	Initialized bool      `json:"initialized"`
	Baseline    string    `json:"baseline"`
	Pending     string    `json:"pending,omitempty"`
	Paused      bool      `json:"paused,omitempty"`
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func (s *Snapshot) Snippet(id string) *Snippet {
	for i := range s.Snippets {
		if s.Snippets[i].ID == id {
			return &s.Snippets[i]
		}
	}
	return nil
}
func (s *Snapshot) Add(name, body string) string {
	order := 0
	for _, n := range s.Snippets {
		order = max(order, n.Order+1)
	}
	now := time.Now()
	id := newID()
	s.Snippets = append(s.Snippets, Snippet{ID: id, Name: name, Body: body, Order: order, Created: now, Updated: now})
	return id
}
func (s *Snapshot) Enable(id string, value bool) {
	if n := s.Snippet(id); n != nil {
		n.Enabled = value
	}
}

// Move inserts a snippet before another; an empty before appends to the list.
func (s *Snapshot) Move(id, before string) error {
	if s.Snippet(id) == nil {
		return errors.New("片段不存在")
	}
	if id == before {
		return nil
	}
	if before != "" && s.Snippet(before) == nil {
		return errors.New("目标片段不存在")
	}
	entries := s.Entries()
	entries = slices.DeleteFunc(entries, func(e Entry) bool { return e.ID == id })
	at := len(entries)
	for i, e := range entries {
		if e.ID == before {
			at = i
			break
		}
	}
	entries = slices.Insert(entries, at, Entry{ID: id})
	for i, e := range entries {
		s.Snippet(e.ID).Order = i
	}
	return nil
}
func (s *Snapshot) Entries() []Entry {
	list := slices.Clone(s.Snippets)
	slices.SortStableFunc(list, func(a, b Snippet) int {
		if a.Order < b.Order {
			return -1
		}
		if a.Order > b.Order {
			return 1
		}
		return 0
	})
	out := make([]Entry, 0, len(list))
	for _, n := range list {
		out = append(out, Entry{ID: n.ID, Name: n.Name, Enabled: n.Enabled})
	}
	return out
}
func (s *Snapshot) Merge() string {
	var parts []string
	for _, e := range s.Entries() {
		if !e.Enabled {
			continue
		}
		body := strings.Trim(s.Snippet(e.ID).Body, "\r\n")
		if strings.TrimSpace(body) != "" {
			parts = append(parts, body)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n\n") + "\n"
}
func Matches(n *Snippet, query string) bool {
	hay := strings.ToLower(n.Name + "\n" + n.Body + "\n" + strings.Join(n.Tags, " "))
	for _, q := range strings.Split(strings.ToLower(query), "|") {
		q = strings.TrimSpace(q)
		if q != "" && strings.Contains(hay, q) {
			return true
		}
	}
	return strings.TrimSpace(query) == ""
}
func (s *Snapshot) Validate() error {
	if s.Version != 2 {
		return errors.New("不支持的指令库版本")
	}
	ids := map[string]bool{}
	for _, n := range s.Snippets {
		if n.ID == "" || ids[n.ID] {
			return errors.New("重复或缺少 ID")
		}
		ids[n.ID] = true
	}
	return nil
}
