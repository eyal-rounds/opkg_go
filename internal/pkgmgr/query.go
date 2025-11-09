package pkgmgr

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/oe-mirrors/opkg_go/internal/config"
	"github.com/oe-mirrors/opkg_go/internal/format"
	"github.com/oe-mirrors/opkg_go/internal/pkgdb"
	"github.com/oe-mirrors/opkg_go/internal/repo"
	"github.com/oe-mirrors/opkg_go/internal/version"
)

// ListOptions controls the behaviour of ListPackages.
type ListOptions struct {
	InstalledOnly    bool
	Patterns         []string
	ShortDescription bool
	IncludeSize      bool
}

// UpgradeCandidate represents an installed package that has a newer version
// available in the configured feeds.
type UpgradeCandidate struct {
	Name        string
	Installed   string
	Available   string
	Description string
}

// UpgradeResult contains the outcome of an upgrade operation for a single
// package.
type UpgradeResult struct {
	Upgrade     UpgradeCandidate
	Destination string
}

func (m *Manager) ensureIndexesLoaded() error {
	if !m.indexesLoaded {
		return errors.New("package indexes not loaded; run 'opkg update' first")
	}
	return nil
}

// ListPackages returns the list of packages matching the provided filters.
func (m *Manager) ListPackages(opts ListOptions) ([]string, error) {
	if opts.InstalledOnly {
		return m.listInstalled(opts)
	}
	if err := m.ensureIndexesLoaded(); err != nil {
		return nil, err
	}
	pkgs := m.indexes.All()
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })

	var lines []string
	for _, pkg := range pkgs {
		if !matchesAny(pkg.Name, opts.Patterns) {
			continue
		}
		desc := pkg.Description
		if opts.ShortDescription {
			desc = firstLine(desc)
		} else {
			desc = strings.ReplaceAll(desc, "\n", " ")
		}
		if desc == "" {
			desc = "(no description)"
		}
		status := ""
		if m.status.Installed(pkg.Name) {
			status = " [installed]"
		}
		if opts.IncludeSize && pkg.Size != "" {
			lines = append(lines, fmt.Sprintf("%s - %s%s (%s)", pkg.Name, desc, status, pkg.Size))
			continue
		}
		lines = append(lines, fmt.Sprintf("%s - %s%s", pkg.Name, desc, status))
	}
	return lines, nil
}

func (m *Manager) listInstalled(opts ListOptions) ([]string, error) {
	entries := m.status.Entries()
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	var lines []string
	for _, entry := range entries {
		if !matchesAny(entry.Name, opts.Patterns) {
			continue
		}
		desc := entry.Raw.Value("Description")
		if opts.ShortDescription {
			desc = firstLine(desc)
		} else {
			desc = strings.ReplaceAll(desc, "\n", " ")
		}
		if desc == "" {
			desc = "(no description)"
		}
		if opts.IncludeSize {
			size := entry.Raw.Value("Installed-Size")
			if size != "" {
				lines = append(lines, fmt.Sprintf("%s - %s (%s)", entry.Name, desc, size))
				continue
			}
		}
		lines = append(lines, fmt.Sprintf("%s - %s", entry.Name, desc))
	}
	return lines, nil
}

// ListUpgradable reports all installed packages that have newer versions
// available. The patterns argument follows the same semantics as ListPackages.
func (m *Manager) ListUpgradable(patterns []string) ([]UpgradeCandidate, error) {
	if err := m.ensureIndexesLoaded(); err != nil {
		return nil, err
	}
	var candidates []UpgradeCandidate
	for _, entry := range m.status.Entries() {
		if !matchesAny(entry.Name, patterns) {
			continue
		}
		pkg, ok := m.indexes.Find(entry.Name)
		if !ok {
			continue
		}
		if version.Compare(entry.Version, pkg.Version) >= 0 {
			continue
		}
		candidates = append(candidates, UpgradeCandidate{
			Name:        entry.Name,
			Installed:   entry.Version,
			Available:   pkg.Version,
			Description: firstLine(pkg.Description),
		})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Name < candidates[j].Name })
	return candidates, nil
}

// Upgrade downloads newer versions for packages that have updates available.
func (m *Manager) Upgrade(ctx context.Context, patterns []string) ([]UpgradeResult, error) {
	candidates, err := m.ListUpgradable(patterns)
	if err != nil {
		return nil, err
	}
	var results []UpgradeResult
	for _, candidate := range candidates {
		dest, err := m.Install(ctx, candidate.Name)
		if err != nil {
			return results, err
		}
		results = append(results, UpgradeResult{Upgrade: candidate, Destination: dest})
	}
	return results, nil
}

// Download retrieves the package archive for the provided package name without
// making any changes to the status database.
func (m *Manager) Download(ctx context.Context, name string) (string, error) {
	return m.Install(ctx, name)
}

// Status returns the status paragraphs for all installed packages matching the
// provided patterns. When no patterns are provided all entries are returned.
func (m *Manager) StatusParagraphs(patterns []string) []pkgdb.Entry {
	if len(patterns) == 0 {
		return m.status.Entries()
	}
	var entries []pkgdb.Entry
	for _, entry := range m.status.Entries() {
		if matchesAny(entry.Name, patterns) {
			entries = append(entries, entry)
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries
}

// Clean removes cached package archives from the cache directory.
func (m *Manager) Clean() error {
	entries, err := os.ReadDir(m.cache)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(m.cache, entry.Name())
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}

// Architectures returns the architectures declared in the configuration file.
func (m *Manager) Architectures() []config.Architecture {
	if m.cfg == nil {
		return nil
	}
	return append([]config.Architecture(nil), m.cfg.Architectures...)
}

func matchesAny(name string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, pattern := range patterns {
		if ok, err := path.Match(pattern, name); err == nil && ok {
			return true
		}
	}
	return false
}

func firstLine(text string) string {
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		return text[:idx]
	}
	return text
}

// ReverseDependencyQuery describes the type of relationship to query for
// reverse lookups such as whatdepends or whatrecommends.
type ReverseDependencyQuery struct {
	Field      string
	IncludeAll bool
	Recursive  bool
	Patterns   []string
}

// ReverseDependencies returns packages that declare a relationship with the
// provided target patterns. Patterns follow shell glob semantics. When
// recursive is enabled the search is extended to packages that depend on the
// matches as well.
func (m *Manager) ReverseDependencies(q ReverseDependencyQuery) ([]string, error) {
	if err := m.ensureIndexesLoaded(); err != nil {
		return nil, err
	}
	if len(q.Patterns) == 0 {
		return nil, errors.New("at least one package name or glob is required")
	}

	universe := m.indexes.All()
	if q.IncludeAll {
		universe = appendMissingInstalled(universe, m.status)
	} else {
		universe = appendMissingInstalled(filterInstalled(universe, m.status), m.status)
	}

	queue := append([]string(nil), q.Patterns...)
	seenTargets := map[string]bool{}
	matched := map[string]bool{}

	for len(queue) > 0 {
		target := queue[0]
		queue = queue[1:]
		if seenTargets[target] {
			continue
		}
		seenTargets[target] = true
		for _, pkg := range universe {
			if matched[pkg.Name] {
				continue
			}
			if relationMatches(pkg.Raw.Value(q.Field), target) {
				matched[pkg.Name] = true
				if q.Recursive {
					queue = append(queue, pkg.Name)
				}
			}
		}
	}

	var result []string
	for name := range matched {
		result = append(result, name)
	}
	sort.Strings(result)
	return result, nil
}

func filterInstalled(pkgs []repo.Package, status *pkgdb.Status) []repo.Package {
	var out []repo.Package
	for _, pkg := range pkgs {
		if status.Installed(pkg.Name) {
			out = append(out, pkg)
		}
	}
	return out
}

// Dependencies returns the relationships declared by the given package.
func (m *Manager) Dependencies(name string) (map[string][]string, error) {
	if err := m.ensureIndexesLoaded(); err != nil {
		return nil, err
	}
	pkg, ok := m.indexes.Find(name)
	if !ok {
		entry, err := m.status.Lookup(name)
		if err != nil {
			return nil, fmt.Errorf("package %s not found", name)
		}
		return dependenciesFromParagraph(entry.Raw), nil
	}
	return dependenciesFromParagraph(pkg.Raw), nil
}

func dependenciesFromParagraph(p format.Paragraph) map[string][]string {
	fields := []string{"Depends", "Pre-Depends", "Recommends", "Suggests", "Provides", "Conflicts", "Replaces"}
	result := make(map[string][]string, len(fields))
	for _, field := range fields {
		if value := p.Value(field); value != "" {
			result[field] = tokensFromRelations(value)
		}
	}
	return result
}

func tokensFromRelations(field string) []string {
	var result []string
	for _, clause := range strings.Split(field, ",") {
		clause = strings.TrimSpace(clause)
		if clause == "" {
			continue
		}
		for _, part := range strings.Split(clause, "|") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			name := part
			if idx := strings.IndexAny(part, " (<>="); idx >= 0 {
				name = strings.TrimSpace(part[:idx])
			}
			result = append(result, name)
		}
	}
	return result
}

func relationMatches(field, pattern string) bool {
	if field == "" {
		return false
	}
	tokens := tokensFromRelations(field)
	for _, token := range tokens {
		if ok, err := path.Match(pattern, token); err == nil && ok {
			return true
		}
	}
	return false
}

func appendMissingInstalled(pkgs []repo.Package, status *pkgdb.Status) []repo.Package {
	seen := map[string]bool{}
	for _, pkg := range pkgs {
		seen[pkg.Name] = true
	}
	for _, entry := range status.Entries() {
		if seen[entry.Name] {
			continue
		}
		pkgs = append(pkgs, repo.Package{
			Name:        entry.Name,
			Version:     entry.Version,
			Description: entry.Raw.Value("Description"),
			Raw:         entry.Raw,
		})
	}
	return pkgs
}

// FindPackages performs a substring search across package names and
// descriptions.
func (m *Manager) FindPackages(pattern string) ([]repo.Package, error) {
	if err := m.ensureIndexesLoaded(); err != nil {
		return nil, err
	}
	pattern = strings.ToLower(pattern)
	var matches []repo.Package
	for _, pkg := range m.indexes.All() {
		if strings.Contains(strings.ToLower(pkg.Name), pattern) || strings.Contains(strings.ToLower(pkg.Description), pattern) {
			matches = append(matches, pkg)
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Name < matches[j].Name })
	return matches, nil
}

// InfoParagraphs returns metadata for packages matching the provided patterns.
func (m *Manager) InfoParagraphs(patterns []string) ([]format.Paragraph, error) {
	if err := m.ensureIndexesLoaded(); err != nil {
		return nil, err
	}
	var paragraphs []format.Paragraph
	seen := map[string]bool{}
	for _, pkg := range m.indexes.All() {
		if !matchesAny(pkg.Name, patterns) {
			continue
		}
		paragraphs = append(paragraphs, pkg.Raw)
		seen[pkg.Name] = true
	}
	// Include installed packages that are missing from the index.
	for _, entry := range m.status.Entries() {
		if seen[entry.Name] {
			continue
		}
		if matchesAny(entry.Name, patterns) {
			paragraphs = append(paragraphs, entry.Raw)
		}
	}
	return paragraphs, nil
}

// GlobStatus returns paragraphs from the status database matching the
// provided patterns. If no patterns are supplied all entries are returned.
func (m *Manager) GlobStatus(patterns []string) []format.Paragraph {
	entries := m.StatusParagraphs(patterns)
	out := make([]format.Paragraph, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.Raw)
	}
	return out
}
