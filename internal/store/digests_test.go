package store

import "testing"

func TestSaveDigestAddsResidualTheme(t *testing.T) {
	st := newTestStore(t)
	f, _, _ := st.AddFeed("https://example.com/feed", "Example", "", "Tech")
	ids := seedArticles(t, st, f.ID, 5)

	// Reference only the first two; the other three must land in "Outros".
	d, err := st.SaveDigest("2026-07-14", "agent", []ThemeInput{
		{Name: "Primary", Summary: "s", ArticleIDs: ids[:2]},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Themes) != 2 {
		t.Fatalf("expected 2 themes (1 + residual), got %d", len(d.Themes))
	}
	residual := d.Themes[1]
	if residual.Name != ResidualThemeName {
		t.Fatalf("expected residual theme %q, got %q", ResidualThemeName, residual.Name)
	}
	if residual.ArticleCount != 3 {
		t.Fatalf("expected 3 residual articles, got %d", residual.ArticleCount)
	}
	// Total coverage: every unread article is in exactly one theme.
	total := d.Themes[0].ArticleCount + residual.ArticleCount
	if total != 5 {
		t.Fatalf("expected full coverage of 5 unread, got %d", total)
	}
}

func TestSaveDigestNoResidualWhenAllReferenced(t *testing.T) {
	st := newTestStore(t)
	f, _, _ := st.AddFeed("https://example.com/feed", "Example", "", "Tech")
	ids := seedArticles(t, st, f.ID, 3)

	d, err := st.SaveDigest("2026-07-14", "", []ThemeInput{{Name: "All", ArticleIDs: ids}})
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Themes) != 1 {
		t.Fatalf("expected no residual theme, got %d themes", len(d.Themes))
	}
}

func TestSaveDigestValidation(t *testing.T) {
	st := newTestStore(t)
	f, _, _ := st.AddFeed("https://example.com/feed", "Example", "", "Tech")
	ids := seedArticles(t, st, f.ID, 2)

	// Non-existent id.
	if _, err := st.SaveDigest("2026-07-14", "", []ThemeInput{{Name: "T", ArticleIDs: []int64{9999}}}); err == nil {
		t.Fatal("expected error for non-existent article")
	}
	// Already-read id.
	if _, err := st.SetRead(ids[:1], true); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SaveDigest("2026-07-14", "", []ThemeInput{{Name: "T", ArticleIDs: ids[:1]}}); err == nil {
		t.Fatal("expected error for already-read article")
	}
}

func TestSaveDigestReplacesExisting(t *testing.T) {
	st := newTestStore(t)
	f, _, _ := st.AddFeed("https://example.com/feed", "Example", "", "Tech")
	ids := seedArticles(t, st, f.ID, 4)

	if _, err := st.SaveDigest("2026-07-14", "", []ThemeInput{{Name: "First", ArticleIDs: ids[:1]}}); err != nil {
		t.Fatal(err)
	}
	d, err := st.SaveDigest("2026-07-14", "", []ThemeInput{{Name: "Second", ArticleIDs: ids[:2]}})
	if err != nil {
		t.Fatal(err)
	}
	if d.Themes[0].Name != "Second" {
		t.Fatalf("expected replaced digest, got theme %q", d.Themes[0].Name)
	}
	// Only one digest row should exist for the date.
	var count int
	_ = st.db.QueryRow(`SELECT COUNT(*) FROM digests WHERE date = ?`, "2026-07-14").Scan(&count)
	if count != 1 {
		t.Fatalf("expected 1 digest row, got %d", count)
	}
}

func TestThemeArticlesAndMarkAllInTheme(t *testing.T) {
	st := newTestStore(t)
	f, _, _ := st.AddFeed("https://example.com/feed", "Example", "", "Tech")
	ids := seedArticles(t, st, f.ID, 4)

	d, err := st.SaveDigest("2026-07-14", "", []ThemeInput{{Name: "Grp", ArticleIDs: ids[:3]}})
	if err != nil {
		t.Fatal(err)
	}
	themeID := d.Themes[0].ID

	arts, err := st.ThemeArticles(themeID)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 3 {
		t.Fatalf("expected 3 articles in theme, got %d", len(arts))
	}

	n, err := st.MarkReadAllInTheme(themeID)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("expected 3 marked read, got %d", n)
	}
	unread, _ := st.CountUnread()
	if unread != 1 {
		t.Fatalf("expected 1 unread left, got %d", unread)
	}
}

func TestGetAndLatestDigest(t *testing.T) {
	st := newTestStore(t)
	f, _, _ := st.AddFeed("https://example.com/feed", "Example", "", "Tech")
	seedArticles(t, st, f.ID, 2)

	if _, err := st.GetDigest("2026-07-14"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if _, err := st.LatestDigestDate(); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for latest, got %v", err)
	}

	_, _ = st.SaveDigest("2026-07-10", "", nil)
	_, _ = st.SaveDigest("2026-07-14", "", nil)
	latest, err := st.LatestDigestDate()
	if err != nil {
		t.Fatal(err)
	}
	if latest != "2026-07-14" {
		t.Fatalf("expected latest 2026-07-14, got %s", latest)
	}

	if _, err := st.ThemeByID(9999); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for missing theme, got %v", err)
	}
}
