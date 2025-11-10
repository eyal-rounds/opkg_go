package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadIncludesRelativeGlobs(t *testing.T) {
	dir := t.TempDir()

	mainCfg := filepath.Join(dir, "opkg.conf")
	if err := os.WriteFile(mainCfg, []byte("include feeds/*.conf\n"), 0o644); err != nil {
		t.Fatalf("write main config: %v", err)
	}

	feedsDir := filepath.Join(dir, "feeds")
	if err := os.Mkdir(feedsDir, 0o755); err != nil {
		t.Fatalf("mkdir feeds dir: %v", err)
	}

	feedCfg := filepath.Join(feedsDir, "base.conf")
	feedData := "src/gz base http://example.invalid/base\n"
	if err := os.WriteFile(feedCfg, []byte(feedData), 0o644); err != nil {
		t.Fatalf("write feed config: %v", err)
	}

	cfg, err := Load(mainCfg)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(cfg.Feeds) != 1 {
		t.Fatalf("expected 1 feed, got %d", len(cfg.Feeds))
	}
	if cfg.Feeds[0].Name != "base" {
		t.Fatalf("unexpected feed name %q", cfg.Feeds[0].Name)
	}
	if cfg.Feeds[0].URI != "http://example.invalid/base" {
		t.Fatalf("unexpected feed URI %q", cfg.Feeds[0].URI)
	}
}

func TestStatusPathOptions(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "opkg.conf")

	contents := "option status_file /var/lib/opkg/status\n" +
		"option status_dir /usr/lib/opkg\n" +
		"dest root /\n"

	if err := os.WriteFile(cfgPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	status, err := cfg.StatusPath()
	if err != nil {
		t.Fatalf("StatusPath returned error: %v", err)
	}
	if status != "/var/lib/opkg/status" {
		t.Fatalf("unexpected status path %q", status)
	}

	delete(cfg.Options, "status_file")
	status, err = cfg.StatusPath()
	if err != nil {
		t.Fatalf("StatusPath without status_file returned error: %v", err)
	}
	if status != filepath.Join("/usr/lib/opkg", "status") {
		t.Fatalf("unexpected fallback status path %q", status)
	}

	delete(cfg.Options, "status_dir")
	status, err = cfg.StatusPath()
	if err != nil {
		t.Fatalf("StatusPath without status_dir returned error: %v", err)
	}
	if status != filepath.Join("/", "usr/lib/opkg/status") {
		t.Fatalf("unexpected dest fallback status path %q", status)
	}
}
