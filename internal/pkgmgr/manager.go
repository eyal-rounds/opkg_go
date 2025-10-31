package pkgmgr

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/oe-mirrors/opkg_go/internal/config"
	"github.com/oe-mirrors/opkg_go/internal/downloader"
	"github.com/oe-mirrors/opkg_go/internal/format"
	"github.com/oe-mirrors/opkg_go/internal/pkgdb"
	"github.com/oe-mirrors/opkg_go/internal/repo"
)

// Manager coordinates package operations by wiring configuration, repository
// metadata and the status database together.
type Manager struct {
	cfg     *config.Config
	client  *downloader.Client
	status  *pkgdb.Status
	indexes repo.IndexSet
	cache   string
}

// New creates a package manager using the provided configuration file.
func New(cfgPath string) (*Manager, error) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}
	cache, err := config.EnsureCacheDir(cfg)
	if err != nil {
		return nil, err
	}
	statusPath, err := cfg.StatusPath()
	var status *pkgdb.Status
	if err != nil {
		status = pkgdb.Empty()
	} else {
		status, err = pkgdb.Load(statusPath)
		if err != nil {
			// When the status file is missing we continue with an empty DB.
			if errors.Is(err, os.ErrNotExist) {
				status = pkgdb.Empty()
			} else {
				return nil, err
			}
		}
	}

	return &Manager{
		cfg:    cfg,
		client: downloader.New(0),
		status: status,
		cache:  cache,
	}, nil
}

// Update refreshes the remote package metadata.
func (m *Manager) Update(ctx context.Context) error {
	indexes, err := repo.Update(ctx, m.cfg, m.cache, m.client)
	if err != nil {
		return err
	}
	m.indexes = repo.NewIndexSet(indexes)
	return nil
}

// List returns a human readable representation of packages available in the
// repositories. When installedOnly is true only packages present in the status
// database are returned.
func (m *Manager) List(installedOnly bool) []string {
	var lines []string
	if installedOnly {
		for _, entry := range m.status.Entries() {
			lines = append(lines, fmt.Sprintf("%s - %s", entry.Name, entry.Version))
		}
		return lines
	}

	for _, pkg := range m.indexes.All() {
		status := ""
		if m.status.Installed(pkg.Name) {
			status = " [installed]"
		}
		desc := strings.ReplaceAll(pkg.Description, "\n", " ")
		lines = append(lines, fmt.Sprintf("%s - %s%s", pkg.Name, desc, status))
	}
	return lines
}

// Info returns detailed information about the provided package name.
func (m *Manager) Info(name string) (string, error) {
	pkg, ok := m.indexes.Find(name)
	if !ok {
		if entry, err := m.status.Lookup(name); err == nil {
			return formatParagraph(entry.Raw), nil
		}
		return "", fmt.Errorf("package %s not found", name)
	}
	return formatParagraph(pkg.Raw), nil
}

// Install downloads the package archive into the cache directory. The Go
// implementation does not attempt to unpack or execute maintainer scripts; it
// focuses on downloading the package and leaving further processing to the
// caller or external tooling.
func (m *Manager) Install(ctx context.Context, name string) (string, error) {
	pkg, ok := m.indexes.Find(name)
	if !ok {
		return "", fmt.Errorf("package %s not available", name)
	}
	if pkg.Filename == "" {
		return "", fmt.Errorf("package %s does not declare a Filename field", name)
	}
	url := strings.TrimSuffix(pkg.Feed.URI, "/") + "/" + strings.TrimPrefix(pkg.Filename, "/")
	dest := filepath.Join(m.cache, filepath.Base(pkg.Filename))
	if err := m.client.DownloadToFile(ctx, url, dest); err != nil {
		return "", err
	}
	return dest, nil
}

func formatParagraph(p format.Paragraph) string {
	var lines []string
	keys := p.Keys()
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("%s: %s", key, strings.ReplaceAll(p.Fields[key], "\n", "\n ")))
	}
	return strings.Join(lines, "\n")
}

// Status returns the current status database.
func (m *Manager) Status() *pkgdb.Status {
	return m.status
}
