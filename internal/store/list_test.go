package store

import "testing"

func TestListArticlesFilters(t *testing.T) {
	st := newTestStore(t)
	goFeed, _, _ := st.AddFeed("https://a/feed", "A", "", "Go")
	phpFeed, _, _ := st.AddFeed("https://b/feed", "B", "", "PHP")
	goIDs := seedArticles(t, st, goFeed.ID, 6)
	seedArticles(t, st, phpFeed.ID, 4)

	// Status + category.
	arts, total, err := st.ListArticles(ArticleFilter{Status: "unread", Category: "Go"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 6 || len(arts) != 6 {
		t.Fatalf("Go unread: total=%d len=%d", total, len(arts))
	}

	// Pagination.
	arts, total, err = st.ListArticles(ArticleFilter{PerPage: 4, Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if total != 10 || len(arts) != 4 {
		t.Fatalf("page1: total=%d len=%d", total, len(arts))
	}
	arts, _, _ = st.ListArticles(ArticleFilter{PerPage: 4, Page: 3})
	if len(arts) != 2 {
		t.Fatalf("page3 should have remainder 2, got %d", len(arts))
	}

	// Read filter after marking some read.
	if _, err := st.SetRead(goIDs[:2], true); err != nil {
		t.Fatal(err)
	}
	_, total, _ = st.ListArticles(ArticleFilter{Status: "read"})
	if total != 2 {
		t.Fatalf("expected 2 read, got %d", total)
	}

	// Starred filter.
	if _, err := st.SetStarred(goIDs[2:3], true); err != nil {
		t.Fatal(err)
	}
	_, total, _ = st.ListArticles(ArticleFilter{Status: "starred"})
	if total != 1 {
		t.Fatalf("expected 1 starred, got %d", total)
	}
}

func TestListArticlesByThemeAndQuery(t *testing.T) {
	st := newTestStore(t)
	f, _, _ := st.AddFeed("https://a/feed", "A", "", "Go")
	ids := seedArticles(t, st, f.ID, 3)
	// Give one article a distinctive title for FTS.
	if _, err := st.db.Exec(`UPDATE articles SET title = 'Symfony deep dive' WHERE id = ?`, ids[0]); err != nil {
		t.Fatal(err)
	}

	d, err := st.SaveDigest("2026-07-14", "", []ThemeInput{{Name: "T", ArticleIDs: ids[:2]}})
	if err != nil {
		t.Fatal(err)
	}
	_, total, err := st.ListArticles(ArticleFilter{ThemeID: d.Themes[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("theme filter: expected 2, got %d", total)
	}

	arts, total, err := st.ListArticles(ArticleFilter{Query: "symfony"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || arts[0].ID != ids[0] {
		t.Fatalf("query filter: total=%d", total)
	}
}

func TestStats(t *testing.T) {
	st := newTestStore(t)
	goFeed, _, _ := st.AddFeed("https://a/feed", "A", "", "Go")
	phpFeed, _, _ := st.AddFeed("https://b/feed", "B", "", "PHP")
	goIDs := seedArticles(t, st, goFeed.ID, 3)
	seedArticles(t, st, phpFeed.ID, 2)

	if _, err := st.SetStarred(goIDs[:1], true); err != nil {
		t.Fatal(err)
	}
	stats, err := st.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.UnreadTotal != 5 {
		t.Fatalf("unread total: %d", stats.UnreadTotal)
	}
	if stats.StarredTotal != 1 {
		t.Fatalf("starred total: %d", stats.StarredTotal)
	}
	if len(stats.ByCategory) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(stats.ByCategory))
	}
}
