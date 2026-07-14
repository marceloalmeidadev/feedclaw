package digest

import (
	"strings"
	"testing"

	"github.com/marceloalmeidadev/feedclaw/internal/store"
)

func TestParseInput(t *testing.T) {
	in, err := ParseInput(strings.NewReader(`{
		"date": "2026-07-14",
		"model_note": "agent",
		"themes": [{"name": "Go", "summary": "s", "article_ids": [1, 2]}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if in.Date != "2026-07-14" || len(in.Themes) != 1 || in.Themes[0].Name != "Go" {
		t.Fatalf("unexpected parse: %+v", in)
	}
	if len(in.Themes[0].ArticleIDs) != 2 {
		t.Fatalf("expected 2 article ids, got %v", in.Themes[0].ArticleIDs)
	}
}

func TestParseInputRejectsUnknownField(t *testing.T) {
	_, err := ParseInput(strings.NewReader(`{"themes": [], "bogus": 1}`))
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestParseInputRequiresThemeName(t *testing.T) {
	_, err := ParseInput(strings.NewReader(`{"themes": [{"summary": "s", "article_ids": [1]}]}`))
	if err == nil {
		t.Fatal("expected error for missing theme name")
	}
}

func TestSave(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	f, _, _ := st.AddFeed("https://example.com/feed", "Example", "", "Tech")
	a := &store.Article{FeedID: f.ID, GUID: "g1", URL: "u", Title: "t"}
	if _, err := st.UpsertArticle(a); err != nil {
		t.Fatal(err)
	}

	in := &Input{Themes: []Theme{{Name: "Grp", Summary: "s", ArticleIDs: []int64{a.ID}}}}
	d, err := Save(st, "2026-07-14", in)
	if err != nil {
		t.Fatal(err)
	}
	if d.Date != "2026-07-14" || len(d.Themes) != 1 {
		t.Fatalf("unexpected digest: %+v", d)
	}
}
