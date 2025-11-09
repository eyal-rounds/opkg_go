package repo

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/oe-mirrors/opkg_go/internal/config"
	"github.com/oe-mirrors/opkg_go/internal/downloader"
	"github.com/oe-mirrors/opkg_go/internal/format"
	"github.com/oe-mirrors/opkg_go/internal/logging"
)

// Package captures the metadata required to perform dependency resolution and
// installation for a single package entry.
type Package struct {
	Name         string
	Version      string
	Architecture string
	Description  string
	Filename     string
	Size         string
	Feed         config.Feed
	Raw          format.Paragraph
}

// Index contains the parsed metadata for a feed.
type Index struct {
	Feed     config.Feed
	Packages map[string]Package
	Updated  time.Time
}

// Update fetches the Packages files for all feeds defined in the configuration
// and stores them inside cacheDir. The function runs downloads concurrently.
func Update(ctx context.Context, cfg *config.Config, cacheDir string, client *downloader.Client) ([]Index, error) {
	if cfg == nil {
		return nil, errors.New("configuration required")
	}
	if client == nil {
		return nil, errors.New("downloader required")
	}

	logging.Debugf("repo: updating %d feeds", len(cfg.Feeds))

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		result   []Index
		firstErr error
	)

	for _, feed := range cfg.Feeds {
		feed := feed
		wg.Add(1)
		go func() {
			defer wg.Done()
			logging.Debugf("repo: fetching feed %s", feed.Name)
			idx, err := fetchFeed(ctx, feed, cacheDir, client)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
					logging.Debugf("repo: feed %s failed: %v", feed.Name, err)
				}
				mu.Unlock()
				return
			}
			logging.Debugf("repo: feed %s loaded with %d packages", feed.Name, len(idx.Packages))
			mu.Lock()
			result = append(result, *idx)
			mu.Unlock()
		}()
	}

	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return result, nil
}

func fetchFeed(ctx context.Context, feed config.Feed, cacheDir string, client *downloader.Client) (*Index, error) {
	if feed.URI == "" {
		return nil, fmt.Errorf("feed %s has empty URI", feed.Name)
	}
	base := strings.TrimSuffix(feed.URI, "/")
	urls := []string{base + "/Packages.gz", base + "/Packages"}
	var data []byte
	var err error
	for _, url := range urls {
		logging.Debugf("repo: attempting %s", url)
		data, err = client.GetBytes(ctx, url)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("fetch feed %s: %w", feed.Name, err)
	}

	// If data is gzipped decompress it.
	if bytes.HasPrefix(data, []byte{0x1f, 0x8b}) {
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("decompress %s: %w", feed.Name, err)
		}
		defer zr.Close()
		data, err = ioReadAll(zr)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", feed.Name, err)
		}
	}

	cf, err := format.ParseControl(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("parse feed %s: %w", feed.Name, err)
	}

	logging.Debugf("repo: parsing feed %s", feed.Name)

	index := Index{
		Feed:     feed,
		Packages: map[string]Package{},
		Updated:  time.Now(),
	}

	for _, paragraph := range cf.Paragraphs {
		name := paragraph.Value("Package")
		if name == "" {
			continue
		}
		index.Packages[name] = Package{
			Name:         name,
			Version:      paragraph.Value("Version"),
			Architecture: paragraph.Value("Architecture"),
			Description:  paragraph.Value("Description"),
			Filename:     paragraph.Value("Filename"),
			Size:         paragraph.Value("Size"),
			Feed:         feed,
			Raw:          paragraph,
		}
	}

	if cacheDir != "" {
		path := filepath.Join(cacheDir, fmt.Sprintf("%s.Packages", feed.Name))
		if err := osWriteFile(path, data, 0o644); err != nil {
			return nil, fmt.Errorf("cache feed %s: %w", feed.Name, err)
		}
		logging.Debugf("repo: cached feed %s at %s", feed.Name, path)
	}

	return &index, nil
}

// IndexSet aggregates multiple indexes, providing helper functions to query
// packages across feeds.
type IndexSet struct {
	indexes []Index
}

// NewIndexSet wraps indexes into a set.
func NewIndexSet(indexes []Index) IndexSet {
	return IndexSet{indexes: indexes}
}

// Find returns the package with the provided name across all feeds.
func (s IndexSet) Find(name string) (Package, bool) {
	for _, idx := range s.indexes {
		if pkg, ok := idx.Packages[name]; ok {
			return pkg, true
		}
	}
	return Package{}, false
}

// All returns a flattened slice of all packages.
func (s IndexSet) All() []Package {
	var out []Package
	for _, idx := range s.indexes {
		for _, pkg := range idx.Packages {
			out = append(out, pkg)
		}
	}
	return out
}

// Helpers extracted for testing.
var (
	ioReadAll   = func(r io.Reader) ([]byte, error) { return io.ReadAll(r) }
	osWriteFile = func(name string, data []byte, perm os.FileMode) error { return os.WriteFile(name, data, perm) }
)
