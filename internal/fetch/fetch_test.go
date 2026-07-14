package fetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/marceloalmeidadev/feedclaw/internal/store"
	"github.com/mmcdole/gofeed"
)

const sampleFeed = `<?xml version="1.0"?>
<rss version="2.0"><channel>
  <title>Test Feed</title>
  <item><title>First</title><link>https://example.com/1</link><guid>g1</guid></item>
  <item><title>Second</title><link>https://example.com/2</link><guid>g2</guid></item>
</channel></rss>`

// newTestFetcher builds a Fetcher wired to a plain HTTP client (bypassing the
// SSRF dialer, which would otherwise refuse httptest's loopback listener) and a
// fresh in-memory store.
func newTestFetcher(t *testing.T, client *http.Client) (*Fetcher, *store.Store) {
	t.Helper()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	f := &Fetcher{
		cfg:    Config{}.withDefaults(),
		store:  st,
		guard:  NewGuard(ModeRestricted, nil),
		client: client,
		parser: gofeed.NewParser(),
	}
	return f, st
}

func TestFetchStores200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"abc"`)
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(sampleFeed))
	}))
	defer srv.Close()

	f, st := newTestFetcher(t, srv.Client())
	feed, _, _ := st.AddFeed(srv.URL, "Test", "", "Tech")

	res, err := f.Run(context.Background(), []*store.Feed{feed})
	if err != nil {
		t.Fatal(err)
	}
	if res[0].NewArticles != 2 {
		t.Fatalf("expected 2 new articles, got %d (err=%s)", res[0].NewArticles, res[0].Err)
	}
	got, _ := st.FeedByID(feed.ID)
	if got.ETag != `"abc"` {
		t.Fatalf("etag not persisted, got %q", got.ETag)
	}
}

func TestFetchConditional304(t *testing.T) {
	var sawINM string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawINM = r.Header.Get("If-None-Match")
		w.WriteHeader(http.StatusNotModified)
	}))
	defer srv.Close()

	f, st := newTestFetcher(t, srv.Client())
	feed, _, _ := st.AddFeed(srv.URL, "Test", "", "Tech")
	_ = st.MarkFetchSuccess(feed.ID, 200, `"abc"`, "")
	feed, _ = st.FeedByID(feed.ID)

	res, err := f.Run(context.Background(), []*store.Feed{feed})
	if err != nil {
		t.Fatal(err)
	}
	if !res[0].NotModified {
		t.Fatalf("expected not_modified, got %+v", res[0])
	}
	if sawINM != `"abc"` {
		t.Fatalf("expected If-None-Match to be sent, got %q", sawINM)
	}
}

func TestFetchServerErrorIncrementsErrorCount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	f, st := newTestFetcher(t, srv.Client())
	feed, _, _ := st.AddFeed(srv.URL, "Test", "", "Tech")

	res, _ := f.Run(context.Background(), []*store.Feed{feed})
	if res[0].Err == "" {
		t.Fatal("expected an error for 500")
	}
	got, _ := st.FeedByID(feed.ID)
	if got.ErrorCount != 1 {
		t.Fatalf("expected error_count 1, got %d", got.ErrorCount)
	}
}

func TestNewAppliesDefaults(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	f := New(st, Config{})
	if f.client == nil || f.guard == nil || f.parser == nil {
		t.Fatal("New must wire client, guard and parser")
	}
	if f.cfg.Workers != 8 || f.cfg.MaxFeedBytes != 2<<20 || f.cfg.MaxArticleBytes != 8<<20 {
		t.Fatalf("defaults not applied: %+v", f.cfg)
	}
}

func TestClientIsGuarded(t *testing.T) {
	client, guard := Client(Config{})
	if client == nil || guard == nil {
		t.Fatal("Client must return a client and guard")
	}
	if guard.Mode != ModeRestricted {
		t.Fatalf("default mode should be restricted, got %q", guard.Mode)
	}
	// The client refuses to reach a loopback listener (SSRF guard active).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	if resp, err := client.Get(srv.URL); err == nil {
		_ = resp.Body.Close()
		t.Fatal("guarded client should refuse loopback target")
	}
}

func TestFetchByteLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(sampleFeed))
	}))
	defer srv.Close()

	f, st := newTestFetcher(t, srv.Client())
	f.cfg.MaxFeedBytes = 10 // smaller than the payload
	feed, _, _ := st.AddFeed(srv.URL, "Test", "", "Tech")

	res, _ := f.Run(context.Background(), []*store.Feed{feed})
	if res[0].Err == "" {
		t.Fatal("expected byte-limit error")
	}
	n, _ := st.CountArticles()
	if n != 0 {
		t.Fatalf("no articles should be stored on limit breach, got %d", n)
	}
}
