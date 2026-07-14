package fetch

import "testing"

// TestEmptyConfigIsRestricted proves the SSRF guard is on for a zero-value
// Config: an empty fetch.Config must yield a restricted guard whose client
// refuses private, loopback and link-local (cloud metadata) targets. This locks
// in the "always on by default" guarantee — no caller can disable the guard by
// forgetting to set SecurityMode.
func TestEmptyConfigIsRestricted(t *testing.T) {
	client, guard := Client(Config{})
	if guard.Mode != ModeRestricted {
		t.Fatalf("empty Config must yield a restricted guard, got %q", guard.Mode)
	}
	for _, target := range []string{
		"http://127.0.0.1:9/",
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.1/",
	} {
		resp, err := client.Get(target)
		if err == nil {
			_ = resp.Body.Close()
			t.Errorf("empty-Config client must refuse %s", target)
		}
	}
}

// TestNewFetcherGuardIsRestricted mirrors the check for the feed fetcher's own
// client (New with a zero SecurityMode).
func TestNewFetcherGuardIsRestricted(t *testing.T) {
	f := New(nil, Config{Workers: 4})
	if f.guard.Mode != ModeRestricted {
		t.Fatalf("New with empty SecurityMode must be restricted, got %q", f.guard.Mode)
	}
	if err := f.guard.CheckURL("http", "example.com"); err != nil {
		t.Fatalf("public host should pass: %v", err)
	}
	// The dialer refuses private IPs (checkIP is exercised in ssrf_test.go).
}
