package opml

import (
	"os"
	"strings"
	"testing"
)

func TestParseFeedlyFixture(t *testing.T) {
	f, err := os.Open("testdata/feedly.opml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	feeds, err := Parse(f)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(feeds) != 4 {
		t.Fatalf("expected 4 feeds, got %d", len(feeds))
	}

	byURL := map[string]Feed{}
	for _, fd := range feeds {
		byURL[fd.XMLURL] = fd
	}

	if got := byURL["https://go.dev/blog/feed.atom"]; got.Category != "Go" || got.Title != "The Go Blog" {
		t.Fatalf("go blog: got category=%q title=%q", got.Category, got.Title)
	}
	if got := byURL["https://symfony.com/blog/rss.xml"]; got.Category != "PHP & Symfony" {
		t.Fatalf("symfony: expected decoded category, got %q", got.Category)
	}
	if got := byURL["https://example.com/uncategorized.xml"]; got.Category != "" {
		t.Fatalf("top-level feed should have empty category, got %q", got.Category)
	}
}

func TestParseRejectsDoctype(t *testing.T) {
	// A billion-laughs style payload must be refused before entity expansion.
	payload := `<?xml version="1.0"?>
<!DOCTYPE opml [<!ENTITY lol "lol">]>
<opml><body><outline xmlUrl="https://x/feed"/></body></opml>`
	_, err := Parse(strings.NewReader(payload))
	if err == nil {
		t.Fatal("expected DOCTYPE to be rejected")
	}
	if !strings.Contains(err.Error(), "DOCTYPE") {
		t.Fatalf("expected DOCTYPE error, got %v", err)
	}
}

func TestParseEmptyBody(t *testing.T) {
	feeds, err := Parse(strings.NewReader(`<opml><body></body></opml>`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(feeds) != 0 {
		t.Fatalf("expected 0 feeds, got %d", len(feeds))
	}
}
