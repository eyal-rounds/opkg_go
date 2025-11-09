package config

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/oe-mirrors/opkg_go/internal/logging"
)

// Feed represents a remote package feed declared in opkg.conf using the
// "src" or "src/gz" directives.
type Feed struct {
	Name string
	URI  string
	Type string
}

// Destination represents a named filesystem destination used by opkg to store
// packages. Only the name and path are required for the Go implementation.
type Destination struct {
	Name string
	Path string
}

// Config stores the parsed opkg configuration. The structure is intentionally
// forgiving so that we can operate on existing configuration files without
// supporting every historical knob.
type Config struct {
	Options       map[string]string
	Feeds         []Feed
	Destinations  []Destination
	Includes      []string
	Architectures []Architecture
}

// Architecture represents an architecture entry declared with the "arch"
// directive in opkg.conf. The priority value follows the semantics of the
// original implementation where lower numbers indicate higher preference.
type Architecture struct {
	Name     string
	Priority int
}

// Load parses the provided configuration file and all includes referenced by
// "include" directives. The parser is whitespace agnostic and ignores empty
// lines or comments (lines starting with "#" or "//").
func Load(path string) (*Config, error) {
	cfg := &Config{Options: map[string]string{}}
	visited := map[string]bool{}

	var load func(string) error
	load = func(p string) error {
		if visited[p] {
			return nil
		}
		visited[p] = true

		logging.Debugf("config: loading file %s", p)

		file, err := os.Open(p)
		if err != nil {
			return fmt.Errorf("open config %s: %w", p, err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			raw := strings.TrimSpace(scanner.Text())
			if raw == "" || strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, "//") {
				continue
			}

			tokens := fields(raw)
			if len(tokens) == 0 {
				continue
			}

			switch tokens[0] {
			case "option":
				if len(tokens) < 3 {
					return fmt.Errorf("%s:%d: option expects key and value", p, lineNo)
				}
				key := tokens[1]
				value := strings.Join(tokens[2:], " ")
				cfg.Options[key] = value
			case "dest":
				if len(tokens) < 3 {
					return fmt.Errorf("%s:%d: dest expects name and path", p, lineNo)
				}
				cfg.Destinations = append(cfg.Destinations, Destination{Name: tokens[1], Path: tokens[2]})
			case "src", "src/gz", "src/sig":
				if len(tokens) < 3 {
					return fmt.Errorf("%s:%d: %s expects name and URI", p, lineNo, tokens[0])
				}
				cfg.Feeds = append(cfg.Feeds, Feed{Name: tokens[1], URI: tokens[2], Type: tokens[0]})
			case "arch":
				if len(tokens) < 2 {
					return fmt.Errorf("%s:%d: arch expects name and optional priority", p, lineNo)
				}
				arch := Architecture{Name: tokens[1]}
				if len(tokens) >= 3 {
					prio, err := strconv.Atoi(tokens[2])
					if err != nil {
						return fmt.Errorf("%s:%d: invalid architecture priority %q", p, lineNo, tokens[2])
					}
					arch.Priority = prio
				}
				cfg.Architectures = append(cfg.Architectures, arch)
			case "include":
				if len(tokens) < 2 {
					return fmt.Errorf("%s:%d: include expects a glob", p, lineNo)
				}
				pattern := tokens[1]
				cfg.Includes = append(cfg.Includes, pattern)
				logging.Debugf("config: discovered include %s from %s", pattern, p)
				matches, err := filepath.Glob(pattern)
				if err != nil {
					return fmt.Errorf("%s:%d: invalid glob: %w", p, lineNo, err)
				}
				if len(matches) == 0 {
					logging.Debugf("config: include pattern %s from %s matched no files", pattern, p)
					continue
				}
				for _, match := range matches {
					logging.Debugf("config: including %s", match)
					if err := load(match); err != nil {
						return err
					}
				}
			default:
				// Keep unknown directives so that higher layers can decide how to
				// handle them. Store the remainder of the line in the options map
				// using the directive name as the key.
				if len(tokens) >= 2 {
					cfg.Options[tokens[0]] = strings.Join(tokens[1:], " ")
					continue
				}
				if strings.Contains(tokens[0], "=") && len(tokens) == 1 {
					parts := strings.SplitN(tokens[0], "=", 2)
					cfg.Options[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
					continue
				}
				return fmt.Errorf("%s:%d: unsupported directive %q", p, lineNo, tokens[0])
			}
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("read config %s: %w", p, err)
		}
		return nil
	}

	if err := load(path); err != nil {
		return nil, err
	}

	logging.Debugf(
		"config: loaded %d options, %d feeds, %d destinations, %d architectures",
		len(cfg.Options), len(cfg.Feeds), len(cfg.Destinations), len(cfg.Architectures),
	)

	return cfg, nil
}

// FindOption returns a configuration value using a case-sensitive key. If the
// key is not found the provided fallback is returned.
func (c *Config) FindOption(key, fallback string) string {
	if c == nil {
		return fallback
	}
	if v, ok := c.Options[key]; ok {
		return v
	}
	return fallback
}

// StatusPath returns the filesystem path to the package status database.
func (c *Config) StatusPath() (string, error) {
	if c == nil {
		return "", errors.New("nil config")
	}
	if path := c.FindOption("status", ""); path != "" {
		return path, nil
	}
	for _, dest := range c.Destinations {
		if dest.Name == "root" {
			return filepath.Join(dest.Path, "usr/lib/opkg/status"), nil
		}
	}
	return "", errors.New("status path not configured")
}

// CacheDir returns the directory used to cache downloaded package archives.
func (c *Config) CacheDir() string {
	if c == nil {
		return ""
	}
	if cache := c.FindOption("cache_dir", ""); cache != "" {
		return cache
	}
	if tmp := c.FindOption("tmp_dir", ""); tmp != "" {
		return tmp
	}
	return "/tmp"
}

// ResolveDest returns the filesystem path for a destination name.
func (c *Config) ResolveDest(name string) (string, error) {
	if c == nil {
		return "", errors.New("nil config")
	}
	for _, dest := range c.Destinations {
		if dest.Name == name {
			return dest.Path, nil
		}
	}
	return "", fmt.Errorf("unknown destination %q", name)
}

// fields is similar to strings.Fields but keeps path-like values intact by
// allowing quoted strings. Only double quotes are supported.
func fields(line string) []string {
	var result []string
	var current strings.Builder
	inQuote := false

	for i := 0; i < len(line); i++ {
		ch := line[i]
		switch ch {
		case '"':
			inQuote = !inQuote
		case ' ', '\t':
			if inQuote {
				current.WriteByte(ch)
			} else if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	return result
}

// EnsureCacheDir creates the cache directory with the provided permissions if
// it does not already exist.
func EnsureCacheDir(cfg *Config) (string, error) {
	if cfg == nil {
		return "", errors.New("nil config")
	}
	cache := cfg.CacheDir()
	if cache == "" {
		return "", errors.New("cache directory not configured")
	}
	if err := os.MkdirAll(cache, fs.ModePerm); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}
	logging.Debugf("config: ensured cache directory %s", cache)
	return cache, nil
}
