package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/oe-mirrors/opkg_go/internal/logging"
)

// Client wraps an http.Client to provide convenient helpers for downloading
// repository metadata and package archives.
type Client struct {
	http    *http.Client
	timeout time.Duration
}

// New creates a downloader with sane defaults.
func New(timeout time.Duration) *Client {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		http: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

// GetBytes fetches the URL and returns the body as a byte slice.
func (c *Client) GetBytes(ctx context.Context, url string) ([]byte, error) {
	if c == nil {
		return nil, fmt.Errorf("nil downloader client")
	}
	logging.Debugf("downloader: fetching %s", url)
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s for %s", resp.Status, url)
	}
	body, err := io.ReadAll(resp.Body)
	if err == nil {
		logging.Debugf("downloader: received %d bytes from %s", len(body), url)
	}
	return body, err
}

// DownloadToFile downloads the content from url and writes it to the provided
// path, creating parent directories as necessary.
func (c *Client) DownloadToFile(ctx context.Context, url, path string) error {
	logging.Debugf("downloader: downloading %s to %s", url, path)
	data, err := c.GetBytes(ctx, url)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("prepare directory: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("commit download: %w", err)
	}
	logging.Debugf("downloader: download completed for %s", path)
	return nil
}
