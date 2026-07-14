package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/marceloalmeidadev/feedclaw/internal/fetch"
	"github.com/marceloalmeidadev/feedclaw/internal/store"
)

func newTestServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return New(st, fetch.Config{}), st
}

func seed(t *testing.T, st *store.Store, n int) (*store.Feed, []int64) {
	t.Helper()
	f, _, err := st.AddFeed("https://example.com/feed", "Example", "", "Tech")
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for i := 0; i < n; i++ {
		pub := time.Now().UTC().Add(-time.Duration(i) * time.Hour)
		a := &store.Article{FeedID: f.ID, GUID: string(rune('a' + i)), URL: "https://example.com/x", Title: "Title", PublishedAt: &pub}
		if _, err := st.UpsertArticle(a); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, a.ID)
	}
	return f, ids
}

func do(t *testing.T, srv *Server, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, r)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	return m
}

func TestFeedsCRUD(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := do(t, srv, http.MethodPost, "/api/feeds", `{"url":"https://a/feed","category":"Go"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("add feed: got %d", rec.Code)
	}
	m := decode(t, rec)
	if m["created"] != true {
		t.Fatalf("expected created=true, got %v", m["created"])
	}

	rec = do(t, srv, http.MethodGet, "/api/feeds", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list feeds: got %d", rec.Code)
	}

	rec = do(t, srv, http.MethodPost, "/api/feeds", `{"category":"x"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty url should be 400, got %d", rec.Code)
	}

	rec = do(t, srv, http.MethodDelete, "/api/feeds/1", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("delete feed: got %d", rec.Code)
	}
	rec = do(t, srv, http.MethodDelete, "/api/feeds/999", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete missing feed should be 404, got %d", rec.Code)
	}
}

func TestListArticlesAndFilters(t *testing.T) {
	srv, st := newTestServer(t)
	_, ids := seed(t, st, 5)

	rec := do(t, srv, http.MethodGet, "/api/articles?status=unread&per_page=2", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	m := decode(t, rec)
	if int(m["total"].(float64)) != 5 {
		t.Fatalf("expected total 5, got %v", m["total"])
	}
	if len(m["articles"].([]any)) != 2 {
		t.Fatalf("expected 2 on page, got %d", len(m["articles"].([]any)))
	}

	// Mark one read, then filter by read.
	if _, err := st.SetRead(ids[:1], true); err != nil {
		t.Fatal(err)
	}
	rec = do(t, srv, http.MethodGet, "/api/articles?status=read", "")
	m = decode(t, rec)
	if int(m["total"].(float64)) != 1 {
		t.Fatalf("expected 1 read, got %v", m["total"])
	}
}

func TestBatchReadAndStar(t *testing.T) {
	srv, st := newTestServer(t)
	_, ids := seed(t, st, 3)

	rec := do(t, srv, http.MethodPatch, "/api/articles/read", `{"ids":[`+itoa(ids[0])+`,`+itoa(ids[1])+`],"read":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch read: %d", rec.Code)
	}
	if int(decode(t, rec)["affected"].(float64)) != 2 {
		t.Fatal("expected 2 affected")
	}

	rec = do(t, srv, http.MethodPatch, "/api/articles/star", `{"ids":[`+itoa(ids[0])+`],"starred":true}`)
	if int(decode(t, rec)["affected"].(float64)) != 1 {
		t.Fatal("expected 1 starred")
	}

	// Malformed JSON -> 400.
	rec = do(t, srv, http.MethodPatch, "/api/articles/read", `{"ids": "notarray"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad body, got %d", rec.Code)
	}
}

func TestGetArticleNotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := do(t, srv, http.MethodGet, "/api/articles/123", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"error"`) {
		t.Fatalf("expected error envelope, got %s", rec.Body.String())
	}
}

func TestDigestsAndStats(t *testing.T) {
	srv, st := newTestServer(t)
	_, ids := seed(t, st, 4)
	if _, err := st.SaveDigest("2026-07-14", "", []store.ThemeInput{{Name: "Grp", ArticleIDs: ids[:2]}}); err != nil {
		t.Fatal(err)
	}

	rec := do(t, srv, http.MethodGet, "/api/digests?date=2026-07-14", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("digests: %d", rec.Code)
	}
	m := decode(t, rec)
	if m["date"] != "2026-07-14" {
		t.Fatalf("unexpected date %v", m["date"])
	}

	// Default (no date) -> latest.
	rec = do(t, srv, http.MethodGet, "/api/digests", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("default digest: %d", rec.Code)
	}

	// Theme articles: theme id 1 is the first saved theme.
	rec = do(t, srv, http.MethodGet, "/api/digests/2026-07-14/themes/1/articles", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("theme articles: %d", rec.Code)
	}
	if len(decode(t, rec)["articles"].([]any)) != 2 {
		t.Fatal("expected 2 articles in theme")
	}

	rec = do(t, srv, http.MethodGet, "/api/stats", "")
	m = decode(t, rec)
	if int(m["unread_total"].(float64)) != 4 {
		t.Fatalf("expected 4 unread, got %v", m["unread_total"])
	}
}

func TestErrorPaths(t *testing.T) {
	srv, _ := newTestServer(t)

	// No digest yet: default and explicit date both 404.
	if rec := do(t, srv, http.MethodGet, "/api/digests", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("empty default digest: got %d", rec.Code)
	}
	if rec := do(t, srv, http.MethodGet, "/api/digests?date=2000-01-01", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("missing digest date: got %d", rec.Code)
	}

	// Bad batch body -> 400.
	if rec := do(t, srv, http.MethodPatch, "/api/articles/star", `{"starred": "no"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad star body: got %d", rec.Code)
	}

	// Invalid path ids -> 400.
	if rec := do(t, srv, http.MethodDelete, "/api/feeds/abc", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad feed id: got %d", rec.Code)
	}
	if rec := do(t, srv, http.MethodGet, "/api/digests/2026-07-14/themes/xyz/articles", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad theme id: got %d", rec.Code)
	}

	// Empty theme (no such theme) returns an empty list, not an error.
	if rec := do(t, srv, http.MethodGet, "/api/digests/2026-07-14/themes/999/articles", ""); rec.Code != http.StatusOK {
		t.Fatalf("unknown theme should be 200 empty, got %d", rec.Code)
	}
}

func TestFetchEmpty(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := do(t, srv, http.MethodPost, "/api/fetch", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("fetch: %d", rec.Code)
	}
	if decode(t, rec)["status"] != "done" {
		t.Fatal("expected status done")
	}
}

func TestFullArticle(t *testing.T) {
	srv, st := newTestServer(t)

	// Missing article -> 404.
	rec := do(t, srv, http.MethodPost, "/api/articles/123/full", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing article: got %d", rec.Code)
	}

	// Existing article whose URL is a loopback address: the SSRF guard blocks
	// extraction, so the handler reports 502 (upstream/extract failure).
	f, _ := seed(t, st, 0)
	a := &store.Article{FeedID: f.ID, GUID: "loopback", URL: "http://127.0.0.1:9/x", Title: "T"}
	if _, err := st.UpsertArticle(a); err != nil {
		t.Fatal(err)
	}
	rec = do(t, srv, http.MethodPost, "/api/articles/"+itoa(a.ID)+"/full", "")
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 on blocked extraction, got %d (%s)", rec.Code, rec.Body.String())
	}

	// Invalid id in path -> 400.
	rec = do(t, srv, http.MethodPost, "/api/articles/abc/full", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad id, got %d", rec.Code)
	}
}

func TestNotFoundRoute(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := do(t, srv, http.MethodGet, "/api/nope", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}
