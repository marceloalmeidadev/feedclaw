package fetch

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheckIPBlocksPrivateRanges(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "127.0.0.53", // loopback
		"10.0.0.5", "192.168.1.1", "172.16.0.1", // RFC1918
		"169.254.169.254", // link-local (cloud metadata)
		"0.0.0.0",         // unspecified
		"::1",             // IPv6 loopback
		"fe80::1",         // IPv6 link-local
		"fc00::1",         // IPv6 unique local
	}
	for _, s := range blocked {
		if err := checkIP(net.ParseIP(s)); err == nil {
			t.Errorf("expected %s to be blocked", s)
		}
	}

	allowed := []string{"8.8.8.8", "1.1.1.1", "93.184.216.34", "2606:4700:4700::1111"}
	for _, s := range allowed {
		if err := checkIP(net.ParseIP(s)); err != nil {
			t.Errorf("expected %s to be allowed, got %v", s, err)
		}
	}
}

func TestCheckURLSchemeAndAllowlist(t *testing.T) {
	restricted := NewGuard(ModeRestricted, nil)
	if err := restricted.CheckURL("ftp", "example.com"); err == nil {
		t.Error("ftp must be rejected")
	}
	if err := restricted.CheckURL("https", "example.com"); err != nil {
		t.Errorf("https public host must pass in restricted: %v", err)
	}

	allow := NewGuard(ModeAllowlist, []string{"example.com"})
	if err := allow.CheckURL("https", "sub.example.com"); err != nil {
		t.Errorf("allowlisted suffix must pass: %v", err)
	}
	if err := allow.CheckURL("https", "evil.com"); err == nil {
		t.Error("non-allowlisted host must be rejected in allowlist mode")
	}
}

// TestDialBlocksLoopback ensures the dialer refuses to connect to a real
// loopback listener even though the DNS name resolves locally.
func TestDialBlocksLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("should never be reached"))
	}))
	defer srv.Close()

	guard := NewGuard(ModeRestricted, nil)
	dial := guard.safeDialContext(0)
	// srv.Listener.Addr() is 127.0.0.1:port — the dialer must reject it.
	_, err := dial(context.Background(), "tcp", srv.Listener.Addr().String())
	if err == nil {
		t.Fatal("expected loopback dial to be blocked")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("expected blocked error, got %v", err)
	}
}

func TestCheckRedirectHopLimit(t *testing.T) {
	guard := NewGuard(ModeRestricted, nil)
	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	via := make([]*http.Request, maxRedirects)
	if err := guard.checkRedirect(req, via); err == nil {
		t.Fatalf("expected redirect limit error after %d hops", maxRedirects)
	}
}
