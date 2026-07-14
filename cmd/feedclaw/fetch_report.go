package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/marceloalmeidadev/feedclaw/internal/fetch"
	"github.com/marceloalmeidadev/feedclaw/internal/store"
)

// failedFeed is one feed that errored during a run.
type failedFeed struct {
	URL        string `json:"url"`
	Title      string `json:"title"`
	Error      string `json:"error"`
	ErrorCount int    `json:"error_count"`
}

// fetchReport is the stable JSON payload written after every fetch run (even on
// failure). The on-exit digest flow reads it to decide whether to proceed.
type fetchReport struct {
	SchemaVersion int          `json:"schema_version"`
	RunID         string       `json:"run_id"`
	StartedAt     string       `json:"started_at"`
	FinishedAt    string       `json:"finished_at"`
	ExitCode      int          `json:"exit_code"`
	FeedsTotal    int          `json:"feeds_total"`
	FeedsOK       int          `json:"feeds_ok"`
	Feeds304      int          `json:"feeds_304"`
	FeedsFailed   int          `json:"feeds_failed"`
	FailedFeeds   []failedFeed `json:"failed_feeds"`
	ArticlesNew   int          `json:"articles_new"`
	UnreadTotal   int          `json:"unread_total"`
}

func newRunID() string {
	var b [10]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "run-" + time.Now().Format("20060102T150405")
	}
	return hex.EncodeToString(b[:])
}

func defaultReportPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "feedclaw-last_run.json"
	}
	return filepath.Join(dir, "feedclaw", "last_run.json")
}

// feedMeta enriches failed feeds in the report with title and error count.
type feedMeta struct {
	title      string
	errorCount int
}

func feedMetaMap(st *store.Store) map[string]feedMeta {
	m := map[string]feedMeta{}
	feeds, err := st.ListFeeds()
	if err != nil {
		return m
	}
	for _, f := range feeds {
		m[f.URL] = feedMeta{title: f.Title, errorCount: f.ErrorCount}
	}
	return m
}

// summarize tallies the results into the report and returns the semantic exit
// code for the run.
func (rep *fetchReport) summarize(results []fetch.Result, meta map[string]feedMeta) int {
	rep.FeedsTotal = len(results)
	for _, r := range results {
		rep.ArticlesNew += r.NewArticles
		switch {
		case r.Err != "":
			rep.FeedsFailed++
			m := meta[r.FeedURL]
			rep.FailedFeeds = append(rep.FailedFeeds, failedFeed{
				URL: r.FeedURL, Title: m.title, Error: r.Err, ErrorCount: m.errorCount,
			})
		case r.NotModified:
			rep.Feeds304++
			rep.FeedsOK++
		default:
			rep.FeedsOK++
		}
	}
	return exitCodeFor(rep)
}

// exitCodeFor maps an aggregated report to its semantic exit code.
func exitCodeFor(rep *fetchReport) int {
	switch {
	case rep.FeedsTotal == 0:
		return exitNothingNew
	case rep.FeedsFailed == rep.FeedsTotal:
		return exitNetworkFail
	case rep.ArticlesNew > 0 && rep.FeedsFailed > 0:
		return exitPartial
	case rep.ArticlesNew > 0:
		return exitOK
	default:
		return exitNothingNew
	}
}

// write persists the report to path, filling FinishedAt and ExitCode. Failures
// to write are non-fatal (the exit code is the primary signal).
func (rep *fetchReport) write(path string, code int) {
	rep.ExitCode = code
	rep.FinishedAt = time.Now().Format(time.RFC3339)
	if rep.FailedFeeds == nil {
		rep.FailedFeeds = []failedFeed{}
	}
	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}
