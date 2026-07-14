package main

import (
	"context"
	"fmt"
	"time"

	"github.com/marceloalmeidadev/feedclaw/internal/fetch"
	"github.com/marceloalmeidadev/feedclaw/internal/store"
	"github.com/spf13/cobra"
)

func fetchCmd() *cobra.Command {
	var (
		feedURL string
		workers int
		report  string
	)
	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch feeds concurrently (conditional requests, SSRF guarded)",
		Long: "Fetch feeds concurrently. Exits with a semantic code the on-exit " +
			"digest pipeline reads: 0 new articles, 10 nothing new, 20 partial " +
			"(some feeds failed but new articles), 30 total network failure, 40 " +
			"config/DB error, 50 another fetch already running.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if report == "" {
				report = defaultReportPath()
			}
			return runFetch(feedURL, workers, report)
		},
	}
	cmd.Flags().StringVar(&feedURL, "feed", "", "fetch only this feed URL")
	cmd.Flags().IntVar(&workers, "workers", 8, "number of concurrent workers")
	cmd.Flags().StringVar(&report, "report", "", "write a JSON run report here (default: <config>/feedclaw/last_run.json)")
	return cmd
}

// runFetch fetches feeds, writes a JSON report (always), and returns an
// exitError carrying the semantic exit code.
func runFetch(feedURL string, workers int, reportPath string) error {
	rep := &fetchReport{SchemaVersion: 1, RunID: newRunID(), StartedAt: time.Now().Format(time.RFC3339)}

	st, err := openStore()
	if err != nil {
		rep.write(reportPath, exitConfigError)
		return &exitError{exitConfigError, fmt.Errorf("open store: %w", err)}
	}
	defer func() { _ = st.Close() }()

	release, err := acquireFetchLock()
	if err != nil {
		rep.UnreadTotal, _ = st.CountUnread()
		rep.write(reportPath, exitLocked)
		return &exitError{exitLocked, err}
	}
	defer release()

	feeds, err := selectFeeds(st, feedURL)
	if err != nil {
		rep.UnreadTotal, _ = st.CountUnread()
		rep.write(reportPath, exitConfigError)
		return &exitError{exitConfigError, err}
	}

	results, err := fetch.New(st, fetch.Config{Workers: workers}).Run(context.Background(), feeds)
	if err != nil {
		rep.UnreadTotal, _ = st.CountUnread()
		rep.write(reportPath, exitConfigError)
		return &exitError{exitConfigError, err}
	}

	code := rep.summarize(results, feedMetaMap(st))
	rep.UnreadTotal, _ = st.CountUnread()

	if flagJSON {
		_ = printJSON(results)
	} else {
		printFetchResults(results)
	}
	rep.write(reportPath, code)

	return exitErrorFor(code, rep.FeedsFailed)
}

// selectFeeds returns the single named feed, or nil for "all enabled feeds".
func selectFeeds(st *store.Store, feedURL string) ([]*store.Feed, error) {
	if feedURL == "" {
		return nil, nil
	}
	f, err := st.FeedByURL(feedURL)
	if err != nil {
		return nil, err
	}
	return []*store.Feed{f}, nil
}

// exitErrorFor turns a semantic code into a return value: nil for success (0),
// a text-carrying error only for genuine failures (30).
func exitErrorFor(code, feedsFailed int) error {
	switch code {
	case exitOK:
		return nil
	case exitNetworkFail:
		return &exitError{code, fmt.Errorf("all %d feeds failed", feedsFailed)}
	default:
		return &exitError{code, nil} // 10 / 20: clean non-zero exit, no error text
	}
}

func printFetchResults(results []fetch.Result) {
	var newTotal, errCount int
	for _, r := range results {
		status := fmt.Sprintf("%d", r.Status)
		switch {
		case r.Err != "":
			errCount++
			status = "ERR"
		case r.NotModified:
			status = "304"
		}
		fmt.Printf("[%4s] +%d %s\n", status, r.NewArticles, r.FeedURL)
		if r.Err != "" {
			fmt.Printf("        %s\n", r.Err)
		}
		newTotal += r.NewArticles
	}
	fmt.Printf("done: %d feeds, %d new articles, %d errors\n", len(results), newTotal, errCount)
}
