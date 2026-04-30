package pins

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

type Store struct {
	mu   sync.Mutex
	path string
	set  map[string]struct{}
}

type fileFormat struct {
	Pinned []string `json:"pinned"`
}

func New() (*Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	dir = filepath.Join(dir, "wifiui")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{path: filepath.Join(dir, "pins.json"), set: map[string]struct{}{}}
	if err := s.load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	var f fileFormat
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ssid := range f.Pinned {
		s.set[ssid] = struct{}{}
	}
	return nil
}

func (s *Store) save() error {
	s.mu.Lock()
	pinned := make([]string, 0, len(s.set))
	for k := range s.set {
		pinned = append(pinned, k)
	}
	s.mu.Unlock()
	sort.Strings(pinned)
	data, err := json.MarshalIndent(fileFormat{Pinned: pinned}, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) Has(ssid string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.set[ssid]
	return ok
}

func (s *Store) All() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.set))
	for k := range s.set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (s *Store) Toggle(ssid string) error {
	s.mu.Lock()
	if _, ok := s.set[ssid]; ok {
		delete(s.set, ssid)
	} else {
		s.set[ssid] = struct{}{}
	}
	s.mu.Unlock()
	return s.save()
}
