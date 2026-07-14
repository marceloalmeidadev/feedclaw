package store

import (
	"testing"
	"time"
)

// newTestStore opens a fresh in-memory database with migrations applied.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestMigrationsApplyAndAreIdempotent(t *testing.T) {
	st := newTestStore(t)
	// Re-running migrate must be a no-op (tables already exist).
	if err := st.migrate(); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	// The FTS table and a couple of core tables must exist.
	for _, tbl := range []string{"feeds", "articles", "articles_fts", "digests"} {
		var name string
		err := st.db.QueryRow(`SELECT name FROM sqlite_master WHERE name = ?`, tbl).Scan(&name)
		if err != nil {
			t.Fatalf("table %s missing: %v", tbl, err)
		}
	}
}

func TestAddFeedIsIdempotent(t *testing.T) {
	st := newTestStore(t)
	f1, created, err := st.AddFeed("https://example.com/feed", "Example", "https://example.com", "Tech")
	if err != nil || !created {
		t.Fatalf("first add: created=%v err=%v", created, err)
	}
	f2, created, err := st.AddFeed("https://example.com/feed", "Other", "", "Other")
	if err != nil {
		t.Fatalf("second add: %v", err)
	}
	if created {
		t.Fatal("second add should not create a new feed")
	}
	if f1.ID != f2.ID {
		t.Fatalf("expected same feed id, got %d and %d", f1.ID, f2.ID)
	}
	if f2.Title != "Example" {
		t.Fatalf("existing feed should keep original title, got %q", f2.Title)
	}
}

func TestUpsertArticleDedup(t *testing.T) {
	st := newTestStore(t)
	f, _, _ := st.AddFeed("https://example.com/feed", "Example", "", "Tech")

	art := &Article{FeedID: f.ID, GUID: "g1", URL: "https://example.com/a", Title: "A"}
	inserted, err := st.UpsertArticle(art)
	if err != nil || !inserted {
		t.Fatalf("first upsert: inserted=%v err=%v", inserted, err)
	}
	// Same (feed_id, guid) must not insert again.
	inserted, err = st.UpsertArticle(&Article{FeedID: f.ID, GUID: "g1", URL: "https://example.com/a", Title: "A changed"})
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if inserted {
		t.Fatal("duplicate guid should not insert")
	}
	n, _ := st.CountArticles()
	if n != 1 {
		t.Fatalf("expected 1 article, got %d", n)
	}
}

func TestFallbackGUIDStable(t *testing.T) {
	a := FallbackGUID("https://x/1", "Title")
	b := FallbackGUID("https://x/1", "Title")
	c := FallbackGUID("https://x/2", "Title")
	if a != b {
		t.Fatal("fallback guid must be deterministic")
	}
	if a == c {
		t.Fatal("different inputs must produce different guids")
	}
}

func TestMarkFetchSuccessResetsErrors(t *testing.T) {
	st := newTestStore(t)
	f, _, _ := st.AddFeed("https://example.com/feed", "Example", "", "Tech")
	if err := st.MarkFetchError(f.ID, 500); err != nil {
		t.Fatal(err)
	}
	if err := st.MarkFetchError(f.ID, 500); err != nil {
		t.Fatal(err)
	}
	got, _ := st.FeedByID(f.ID)
	if got.ErrorCount != 2 {
		t.Fatalf("expected error_count 2, got %d", got.ErrorCount)
	}
	if err := st.MarkFetchSuccess(f.ID, 200, "etag123", "Mon, 01 Jan 2026 00:00:00 GMT"); err != nil {
		t.Fatal(err)
	}
	got, _ = st.FeedByID(f.ID)
	if got.ErrorCount != 0 {
		t.Fatalf("success should reset error_count, got %d", got.ErrorCount)
	}
	if got.ETag != "etag123" {
		t.Fatalf("etag not persisted, got %q", got.ETag)
	}
}

func TestFTSSyncOnInsert(t *testing.T) {
	st := newTestStore(t)
	f, _, _ := st.AddFeed("https://example.com/feed", "Example", "", "Tech")
	pub := time.Now().UTC()
	_, err := st.UpsertArticle(&Article{
		FeedID: f.ID, GUID: "g1", URL: "https://example.com/a",
		Title: "Golang concurrency patterns", Summary: "channels and errgroup", PublishedAt: &pub,
	})
	if err != nil {
		t.Fatal(err)
	}
	var rowid int64
	err = st.db.QueryRow(`SELECT rowid FROM articles_fts WHERE articles_fts MATCH ?`, "concurrency").Scan(&rowid)
	if err != nil {
		t.Fatalf("fts match failed: %v", err)
	}
	if rowid == 0 {
		t.Fatal("expected an fts hit")
	}
}

func TestRemoveFeedCascades(t *testing.T) {
	st := newTestStore(t)
	f, _, _ := st.AddFeed("https://example.com/feed", "Example", "", "Tech")
	_, _ = st.UpsertArticle(&Article{FeedID: f.ID, GUID: "g1", URL: "u", Title: "t"})
	if err := st.RemoveFeed("https://example.com/feed"); err != nil {
		t.Fatal(err)
	}
	n, _ := st.CountArticles()
	if n != 0 {
		t.Fatalf("articles should cascade-delete, got %d", n)
	}
	if err := st.RemoveFeed("https://missing"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
