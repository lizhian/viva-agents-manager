package instructions

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestLegacyFoldersFlattenWithoutChangingOutput(t *testing.T) {
	s := fixture(t)
	const old = `{"version":1,"snippets":[{"id":"root","name":"Root","body":"R","order":0,"enabled":true,"pinned":true},{"id":"child","name":"Child","body":"C","order":3,"folder":"parent","enabled":true},{"id":"nested","name":"Nested","body":"N","order":9,"folder":"inner","tags":["legacy"],"enabled":false}],"folders":[{"id":"parent","name":"P","order":1},{"id":"inner","name":"I","parent":"parent","order":0}],"initialized":true}`
	var legacy map[string]json.RawMessage
	if err := json.Unmarshal([]byte(old), &legacy); err != nil {
		t.Fatal(err)
	}
	expected := "C\n\nR\n"
	legacy["baseline"], _ = json.Marshal(digest([]byte(expected)))
	data, _ := json.Marshal(legacy)
	if err := atomicWrite(s.path(), data); err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(s.Config.Target, []byte(expected)); err != nil {
		t.Fatal(err)
	}
	migrated, err := Open(s.Config)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(s.path())
	if string(before) != string(data) {
		t.Fatal("Open must not write before successful initialization")
	}
	entries := migrated.Data.Entries()
	if len(entries) != 3 || entries[0].ID != "nested" || entries[1].ID != "child" || entries[2].ID != "root" {
		t.Fatalf("lost legacy traversal order: %+v", entries)
	}
	if migrated.Data.Snippet("nested").Enabled || migrated.Data.Snippet("nested").Tags[0] != "legacy" {
		t.Fatal("lost metadata")
	}
	if err := migrated.Initialize(); err != nil {
		t.Fatal(err)
	}
	if output(t, migrated) != expected {
		t.Fatal("migration changed generated instructions")
	}
	saved, _ := os.ReadFile(s.path())
	for _, field := range []string{`"folders"`, `"folder"`, `"pinned"`} {
		if strings.Contains(string(saved), field) {
			t.Fatalf("obsolete metadata persisted: %s", field)
		}
	}
	restarted, err := Open(s.Config)
	if err != nil {
		t.Fatal(err)
	}
	if restarted.Data.Version != 2 || len(restarted.Data.Snippets) != 3 || restarted.Data.Merge() != expected {
		t.Fatal("migration not stable across restart")
	}
}
func TestMalformedLegacyHierarchyDoesNotOverwriteFiles(t *testing.T) {
	s := fixture(t)
	data := []byte(`{"version":1,"folders":[{"id":"a","parent":"b"},{"id":"b","parent":"a"}],"snippets":[]}`)
	atomicWrite(s.path(), data)
	atomicWrite(s.Config.Target, []byte("untouched"))
	broken, err := Open(s.Config)
	if err == nil {
		t.Fatal("accepted legacy cycle")
	}
	if broken.Save() == nil {
		t.Fatal("saved invalid library")
	}
	saved, _ := os.ReadFile(s.path())
	if string(saved) != string(data) || output(t, s) != "untouched" {
		t.Fatal("modified files after failed migration")
	}
}
