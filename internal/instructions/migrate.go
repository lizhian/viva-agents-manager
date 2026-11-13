package instructions

import (
	"encoding/json"
	"errors"
	"slices"
)

// migrateV1 is a read-only adapter for the former folder library. Runtime state
// is flat; a later successful save writes v2 without folders or pinned metadata.
func migrateV1(data []byte, s *Snapshot) error {
	var old struct {
		Snippets []struct {
			ID, Folder string
			Order      int
		} `json:"snippets"`
		Folders []struct {
			ID, Parent string
			Order      int
		} `json:"folders"`
	}
	if err := json.Unmarshal(data, &old); err != nil {
		return err
	}
	parent := map[string]string{}
	seen := map[string]bool{}
	for _, f := range old.Folders {
		if f.ID == "" || seen[f.ID] {
			return errors.New("旧指令库的文件夹 ID 无效")
		}
		seen[f.ID] = true
		parent[f.ID] = f.Parent
	}
	for _, f := range old.Folders {
		chain := map[string]bool{f.ID: true}
		for at := f.Parent; at != ""; {
			next, exists := parent[at]
			if !exists || chain[at] {
				return errors.New("旧指令库的文件夹层级无效")
			}
			chain[at] = true
			at = next
		}
	}
	for _, n := range old.Snippets {
		if n.ID == "" || seen[n.ID] {
			return errors.New("旧指令库的片段 ID 无效")
		}
		seen[n.ID] = true
		if n.Folder != "" {
			if _, ok := parent[n.Folder]; !ok {
				return errors.New("旧片段所在的文件夹不存在")
			}
		}
	}
	// Match the former merge traversal, deliberately ignoring pinned display order.
	slices.SortStableFunc(old.Folders, func(a, b struct {
		ID, Parent string
		Order      int
	}) int {
		return a.Order - b.Order
	})
	slices.SortStableFunc(old.Snippets, func(a, b struct {
		ID, Folder string
		Order      int
	}) int {
		return a.Order - b.Order
	})
	order := 0
	var walk func(string)
	walk = func(id string) {
		for _, f := range old.Folders {
			if f.Parent == id {
				walk(f.ID)
			}
		}
		for _, n := range old.Snippets {
			if n.Folder == id {
				snippet := s.Snippet(n.ID)
				if snippet != nil {
					snippet.Order = order
					order++
				}
			}
		}
	}
	walk("")
	s.Version = 2
	return nil
}
