// Package readability downloads an article's HTML through the SSRF-guarded HTTP
// client and extracts a clean, reader-mode version of its main content.
package readability

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	goreadability "github.com/go-shiori/go-readability"
)

// Extract fetches rawURL with the provided (SSRF-guarded) client, enforces the
// byte cap, and returns the extracted article HTML. The guard's dialer blocks
// private/loopback targets, including across redirects; here we additionally
// reject non-http(s) schemes up front.
func Extract(ctx context.Context, client *http.Client, rawURL string, maxBytes int64) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	if maxBytes <= 0 {
		maxBytes = 8 << 20
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	if int64(len(data)) > maxBytes {
		return "", fmt.Errorf("article exceeds max_article_bytes (%d)", maxBytes)
	}

	article, err := goreadability.FromReader(bytes.NewReader(data), u)
	if err != nil {
		return "", fmt.Errorf("extract: %w", err)
	}
	return article.Content, nil
}
