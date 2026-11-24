package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/sreenivas-sadhu-prabhakara/sec-reportorial-autopilot-gis-afs/internal/secfiling"
)

// Store keeps the register in memory and on disk in one plain JSON file.
//
// There is no database and no account: the file sits wherever the user points
// -data at, so it can live in the folder that is already being backed up.
// Writes go to a temporary file in the same directory and are then renamed
// over the old one, so a crash or a full disk leaves the previous good file
// intact rather than a half-written one.
type Store struct {
	mu   sync.RWMutex
	path string
	book *secfiling.Book
}

// OpenStore loads the data file, or starts an empty register if there is not
// one yet. It does not create the file until something is saved.
func OpenStore(path string) (*Store, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("data file %s: %w", path, err)
	}
	s := &Store{path: absolute, book: secfiling.New()}

	raw, err := os.ReadFile(absolute)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not read the data file %s: %w", absolute, err)
	}
	if len(raw) == 0 {
		return s, nil
	}
	loaded := secfiling.New()
	if err := json.Unmarshal(raw, loaded); err != nil {
		return nil, fmt.Errorf("the data file %s is not valid JSON: %w", absolute, err)
	}
	loaded.Normalise()
	s.book = loaded
	return s, nil
}

// Path is where the data file lives, shown in the interface so the user always
// knows where their register is.
func (s *Store) Path() string { return s.path }

// Read runs fn against the register under a read lock. fn must not keep the
// pointer or change anything behind it.
func (s *Store) Read(fn func(b *secfiling.Book)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	fn(s.book)
}

// Write runs fn against the register under a write lock and saves the file if
// fn succeeds. Nothing is saved if fn fails.
func (s *Store) Write(fn func(b *secfiling.Book) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := fn(s.book); err != nil {
		return err
	}
	s.book.Normalise()
	return s.save()
}

// save writes the data file atomically: a temporary file in the same
// directory, flushed to disk, then renamed over the target.
func (s *Store) save() error {
	raw, err := json.MarshalIndent(s.book, "", "  ")
	if err != nil {
		return fmt.Errorf("could not encode the register: %w", err)
	}
	raw = append(raw, '\n')

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("could not create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(s.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("could not write beside %s: %w", s.path, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // harmless once the rename has happened

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("could not set permissions on %s: %w", tmpName, err)
	}
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return fmt.Errorf("could not write %s: %w", tmpName, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("could not flush %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("could not close %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("could not replace %s: %w", s.path, err)
	}
	return nil
}
