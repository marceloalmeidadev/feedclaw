package store

import (
	"testing"
	"time"
)

// seedArticles inserts n articles for a feed with staggered publish times
// (article i published i days ago) and returns their IDs in insert order.
func seedArticles(t *testing.T, st *Store, feedID int64, n int) []int64 {
	t.Helper()
	var ids []int64
	for i := 0; i < n; i++ {
		pub := time.Now().UTC().Add(-time.Duration(i) * 24 * time.Hour)
		a := &Article{
			FeedID:      feedID,
			GUID:        "g" + time.Duration(i).String(),
			URL:         "https://example.com/a" + time.Duration(i).String(),
			Title:       "Article " + time.Duration(i).String(),
			PublishedAt: &pub,
		}
		if _, err := st.UpsertArticle(a); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, a.ID)
	}
	return ids
}

func TestListUnreadSinceFilter(t *testing.T) {
	st := newTestStore(t)
	f, _, _ := st.AddFeed("https://example.com/feed", "Example", "", "Tech")
	seedArticles(t, st, f.ID, 10) // published 0..9 days ago

	// Last 3 days should include articles published 0,1,2 days ago = 3.
	got, err := st.ListUnread(UnreadFilter{Since: 3 * 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 recent unread, got %d", len(got))
	}
	// Ordered newest first.
	if got[0].PublishedAt.Before(*got[1].PublishedAt) {
		t.Fatal("expected newest-first ordering")
	}
}

func TestListUnreadCategoryFilter(t *testing.T) {
	st := newTestStore(t)
	go1, _, _ := st.AddFeed("https://a/feed", "A", "", "Go")
	php, _, _ := st.AddFeed("https://b/feed", "B", "", "PHP")
	seedArticles(t, st, go1.ID, 2)
	seedArticles(t, st, php.ID, 3)

	got, err := st.ListUnread(UnreadFilter{Category: "PHP"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 PHP articles, got %d", len(got))
	}
	for _, a := range got {
		if a.Category != "PHP" {
			t.Fatalf("unexpected category %q", a.Category)
		}
	}
}

func TestSetReadBatchAndUnread(t *testing.T) {
	st := newTestStore(t)
	f, _, _ := st.AddFeed("https://example.com/feed", "Example", "", "Tech")
	ids := seedArticles(t, st, f.ID, 5)

	n, err := st.SetRead(ids[:3], true)
	if err != nil || n != 3 {
		t.Fatalf("SetRead: n=%d err=%v", n, err)
	}
	unread, _ := st.CountUnread()
	if unread != 2 {
		t.Fatalf("expected 2 unread, got %d", unread)
	}
	// Marking read again is idempotent (only affects still-unread rows).
	n, _ = st.SetRead(ids[:3], true)
	if n != 0 {
		t.Fatalf("re-marking read should affect 0 rows, got %d", n)
	}
	// Unmark one.
	n, _ = st.SetRead(ids[:1], false)
	if n != 1 {
		t.Fatalf("SetRead unread: expected 1, got %d", n)
	}
	unread, _ = st.CountUnread()
	if unread != 3 {
		t.Fatalf("expected 3 unread, got %d", unread)
	}
}

func TestMarkReadOlderThan(t *testing.T) {
	st := newTestStore(t)
	f, _, _ := st.AddFeed("https://example.com/feed", "Example", "", "Tech")
	seedArticles(t, st, f.ID, 10) // 0..9 days ago

	n, err := st.MarkReadOlderThan(5 * 24 * time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	// Articles published 5,6,7,8,9 days ago = 5 older than the cutoff.
	if n != 5 {
		t.Fatalf("expected 5 marked, got %d", n)
	}
}

func TestSetStarred(t *testing.T) {
	st := newTestStore(t)
	f, _, _ := st.AddFeed("https://example.com/feed", "Example", "", "Tech")
	ids := seedArticles(t, st, f.ID, 3)

	if _, err := st.SetStarred(ids[:2], true); err != nil {
		t.Fatal(err)
	}
	a, _ := st.ArticleByID(ids[0])
	if !a.Starred {
		t.Fatal("expected article to be starred")
	}
	if _, err := st.SetStarred(ids[:1], false); err != nil {
		t.Fatal(err)
	}
	a, _ = st.ArticleByID(ids[0])
	if a.Starred {
		t.Fatal("expected article to be unstarred")
	}
}

func TestSetFullContentAndFTS(t *testing.T) {
	st := newTestStore(t)
	f, _, _ := st.AddFeed("https://example.com/feed", "Example", "", "Tech")
	ids := seedArticles(t, st, f.ID, 1)

	if err := st.SetFullContent(ids[0], "the quick brown fox jumps"); err != nil {
		t.Fatal(err)
	}
	a, _ := st.ArticleByID(ids[0])
	if a.FullContent != "the quick brown fox jumps" {
		t.Fatalf("full_content not persisted: %q", a.FullContent)
	}
	// full_content is indexed by FTS (via the update trigger).
	res, err := st.Search("brown", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].ID != ids[0] {
		t.Fatalf("expected fts hit on full_content, got %d results", len(res))
	}
	if err := st.SetFullContent(9999, "x"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for missing id, got %v", err)
	}
}

func TestQueryEdgeCases(t *testing.T) {
	st := newTestStore(t)
	f, _, _ := st.AddFeed("https://example.com/feed", "Example", "", "Tech")
	ids := seedArticles(t, st, f.ID, 3)

	// Empty id slices are no-ops.
	if n, err := st.SetRead(nil, true); err != nil || n != 0 {
		t.Fatalf("SetRead(nil): n=%d err=%v", n, err)
	}
	if n, err := st.SetStarred(nil, true); err != nil || n != 0 {
		t.Fatalf("SetStarred(nil): n=%d err=%v", n, err)
	}
	// Blank search returns no results without touching FTS.
	if res, err := st.Search("   ", 10); err != nil || res != nil {
		t.Fatalf("blank search: res=%v err=%v", res, err)
	}
	// Missing article.
	if _, err := st.ArticleByID(999999); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	// Limit caps the unread listing.
	got, err := st.ListUnread(UnreadFilter{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected limit 2, got %d", len(got))
	}
	_ = ids
}

func TestSearchTitle(t *testing.T) {
	st := newTestStore(t)
	f, _, _ := st.AddFeed("https://example.com/feed", "Example", "", "Tech")
	pub := time.Now().UTC()
	_, _ = st.UpsertArticle(&Article{FeedID: f.ID, GUID: "x", URL: "u", Title: "Symfony 8 release notes", PublishedAt: &pub})

	res, err := st.Search("symfony", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	// Query with special characters must not blow up FTS (tokens are quoted).
	if _, err := st.Search("c++ : go-lang", 10); err != nil {
		t.Fatalf("special-char query should be safe: %v", err)
	}
}
