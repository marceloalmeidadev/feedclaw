package main

import "testing"

// TestLoadOPMLBlocksSSRF verifies that fetching an OPML document by URL goes
// through the SSRF-guarded client: private, loopback and link-local (cloud
// metadata) targets must be refused before any body is read.
func TestLoadOPMLBlocksSSRF(t *testing.T) {
	blocked := []string{
		"http://127.0.0.1:9/feeds.opml",            // loopback
		"http://169.254.169.254/latest/meta-data/", // cloud metadata (link-local)
		"http://10.0.0.1/feeds.opml",               // RFC1918
		"http://[::1]:9/feeds.opml",                // IPv6 loopback
	}
	for _, target := range blocked {
		if _, err := loadOPML(target); err == nil {
			t.Errorf("loadOPML(%q) must be blocked by the SSRF guard", target)
		}
	}
}
