package readability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const articleHTML = `<!DOCTYPE html><html><head><title>Test</title></head>
<body>
  <nav>menu junk</nav>
  <article>
    <h1>The Main Headline</h1>
    <p>This is the first substantial paragraph of the article body, long enough
    that the readability heuristics treat it as primary content rather than
    boilerplate navigation or advertising material.</p>
    <p>A second paragraph continues the main content with more meaningful prose
    so the extractor has a clear main region to select from the document.</p>
  </article>
  <footer>copyright junk</footer>
</body></html>`

func TestExtract(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(articleHTML))
	}))
	defer srv.Close()

	// Use a plain client: httptest binds to loopback, which the SSRF-guarded
	// client would (correctly) refuse. The guard itself is covered in fetch.
	got, err := Extract(context.Background(), srv.Client(), srv.URL, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "substantial paragraph") {
		t.Fatalf("extracted content missing main text: %q", got)
	}
	if strings.Contains(got, "menu junk") {
		t.Error("boilerplate navigation should be stripped")
	}
}

func TestExtractRejectsNonHTTPScheme(t *testing.T) {
	_, err := Extract(context.Background(), http.DefaultClient, "file:///etc/passwd", 0)
	if err == nil {
		t.Fatal("expected non-http scheme to be rejected")
	}
}

func TestExtractNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := Extract(context.Background(), srv.Client(), srv.URL, 0)
	if err == nil {
		t.Fatal("expected error for non-2xx status")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected status in error, got %v", err)
	}
}

func TestExtractByteLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(articleHTML))
	}))
	defer srv.Close()

	_, err := Extract(context.Background(), srv.Client(), srv.URL, 10)
	if err == nil {
		t.Fatal("expected byte-limit error")
	}
	if !strings.Contains(err.Error(), "max_article_bytes") {
		t.Fatalf("expected byte-limit error, got %v", err)
	}
}
