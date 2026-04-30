package pins

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func newAt(t *testing.T) *Store {
	t.Helper()
	return &Store{
		path: filepath.Join(t.TempDir(), "pins.json"),
		set:  map[string]struct{}{},
	}
}

func TestStoreToggle(t *testing.T) {
	s := newAt(t)
	if s.Has("foo") {
		t.Fatal("expected empty store to not contain foo")
	}
	if err := s.Toggle("foo"); err != nil {
		t.Fatal(err)
	}
	if !s.Has("foo") {
		t.Fatal("expected foo pinned after toggle")
	}
	if err := s.Toggle("foo"); err != nil {
		t.Fatal(err)
	}
	if s.Has("foo") {
		t.Fatal("expected foo unpinned after second toggle")
	}
}

func TestStoreAllSorted(t *testing.T) {
	s := newAt(t)
	for _, ssid := range []string{"zebra", "alpha", "mango"} {
		if err := s.Toggle(ssid); err != nil {
			t.Fatal(err)
		}
	}
	got := s.All()
	want := []string{"alpha", "mango", "zebra"}
	if len(got) != len(want) {
		t.Fatalf("len(got) = %d", len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d]=%q, want %q", i, got[i], want[i])
		}
	}
}

func TestStorePersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pins.json")

	s1 := &Store{path: path, set: map[string]struct{}{}}
	if err := s1.Toggle("home wifi"); err != nil {
		t.Fatal(err)
	}
	if err := s1.Toggle("iphone"); err != nil {
		t.Fatal(err)
	}

	s2 := &Store{path: path, set: map[string]struct{}{}}
	if err := s2.load(); err != nil {
		t.Fatal(err)
	}
	if !s2.Has("home wifi") || !s2.Has("iphone") {
		t.Errorf("expected both pinned after reload, got %v", s2.All())
	}
}

func TestStoreLoadMissing(t *testing.T) {
	s := &Store{
		path: filepath.Join(t.TempDir(), "missing.json"),
		set:  map[string]struct{}{},
	}
	err := s.load()
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected os.ErrNotExist, got %v", err)
	}
}

func TestStoreFileFormat(t *testing.T) {
	s := newAt(t)
	if err := s.Toggle("foo"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatal(err)
	}
	var f fileFormat
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Pinned) != 1 || f.Pinned[0] != "foo" {
		t.Errorf("file = %s, want pinned=[foo]", data)
	}
}
