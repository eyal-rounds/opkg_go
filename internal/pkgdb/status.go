package pkgdb

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/oe-mirrors/opkg_go/internal/format"
)

// Entry represents a package stored in the status database.
type Entry struct {
	Name         string
	Version      string
	Architecture string
	Status       string
	Raw          format.Paragraph
}

// Status wraps the parsed status database. The structure is safe for
// concurrent readers.
type Status struct {
	path   string
	mu     sync.RWMutex
	byName map[string]Entry
}

// Load reads the status database from disk.
func Load(path string) (*Status, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read status: %w", err)
	}
	cf, err := format.ParseControl(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("parse status: %w", err)
	}
	status := &Status{path: path, byName: map[string]Entry{}}
	for _, paragraph := range cf.Paragraphs {
		name := paragraph.Value("Package")
		if name == "" {
			continue
		}
		status.byName[name] = Entry{
			Name:         name,
			Version:      paragraph.Value("Version"),
			Architecture: paragraph.Value("Architecture"),
			Status:       paragraph.Value("Status"),
			Raw:          paragraph,
		}
	}
	return status, nil
}

// Empty returns a Status instance without backing storage. Useful for systems
// that have not installed any packages yet.
func Empty() *Status {
	return &Status{byName: map[string]Entry{}}
}

// Installed reports whether the given package is installed according to the
// status database.
func (s *Status) Installed(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.byName[name]
	if !ok {
		return false
	}
	return strings.Contains(entry.Status, "installed")
}

// Entries returns a copy of all entries stored in the database.
func (s *Status) Entries() []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Entry, 0, len(s.byName))
	for _, entry := range s.byName {
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Path returns the underlying status file path.
func (s *Status) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// ErrNotFound is returned when a package is not present in the database.
var ErrNotFound = errors.New("package not found")

// Lookup retrieves a package from the status database.
func (s *Status) Lookup(name string) (Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if entry, ok := s.byName[name]; ok {
		return entry, nil
	}
	return Entry{}, ErrNotFound
}
