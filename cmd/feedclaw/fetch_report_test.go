package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/marceloalmeidadev/feedclaw/internal/fetch"
)

func TestExitCodeFor(t *testing.T) {
	cases := []struct {
		name string
		rep  fetchReport
		want int
	}{
		{"no feeds", fetchReport{FeedsTotal: 0}, exitNothingNew},
		{"all failed", fetchReport{FeedsTotal: 3, FeedsFailed: 3}, exitNetworkFail},
		{"partial with new", fetchReport{FeedsTotal: 3, FeedsFailed: 1, ArticlesNew: 5}, exitPartial},
		{"new, no failures", fetchReport{FeedsTotal: 3, ArticlesNew: 5}, exitOK},
		{"nothing new", fetchReport{FeedsTotal: 3, ArticlesNew: 0}, exitNothingNew},
		{"some failed, nothing new", fetchReport{FeedsTotal: 3, FeedsFailed: 1, ArticlesNew: 0}, exitNothingNew},
	}
	for _, c := range cases {
		if got := exitCodeFor(&c.rep); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}

func TestSummarize(t *testing.T) {
	results := []fetch.Result{
		{FeedURL: "a", NewArticles: 3},
		{FeedURL: "b", NotModified: true},
		{FeedURL: "c", Err: "boom"},
	}
	meta := map[string]feedMeta{"c": {title: "C feed", errorCount: 2}}
	rep := &fetchReport{}

	if code := rep.summarize(results, meta); code != exitPartial {
		t.Fatalf("expected partial (20), got %d", code)
	}
	if rep.FeedsTotal != 3 || rep.FeedsOK != 2 || rep.Feeds304 != 1 || rep.FeedsFailed != 1 || rep.ArticlesNew != 3 {
		t.Fatalf("unexpected tallies: %+v", rep)
	}
	if len(rep.FailedFeeds) != 1 {
		t.Fatalf("expected 1 failed feed, got %d", len(rep.FailedFeeds))
	}
	if f := rep.FailedFeeds[0]; f.URL != "c" || f.Title != "C feed" || f.ErrorCount != 2 || f.Error != "boom" {
		t.Fatalf("failed feed not enriched: %+v", f)
	}
}

func TestReportWriteValidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "last_run.json")
	rep := &fetchReport{SchemaVersion: 1, RunID: newRunID()}
	rep.write(path, exitOK)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got fetchReport
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("report is not valid JSON: %v", err)
	}
	if got.SchemaVersion != 1 || got.ExitCode != exitOK || got.FinishedAt == "" {
		t.Fatalf("unexpected report: %+v", got)
	}
	// failed_feeds must be [] not null so consumers can iterate safely.
	if got.FailedFeeds == nil {
		t.Fatal("failed_feeds should serialize as [], not null")
	}
}
