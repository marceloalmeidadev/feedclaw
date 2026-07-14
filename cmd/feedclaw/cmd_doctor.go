package main

import (
	"fmt"

	"github.com/marceloalmeidadev/feedclaw/internal/store"
	"github.com/spf13/cobra"
)

// errorThreshold is the consecutive-failure count at which a feed is flagged
// (never auto-removed) in doctor output and the UI.
const errorThreshold = 10

type unhealthyFeed struct {
	ID         int64  `json:"id"`
	URL        string `json:"url"`
	ErrorCount int    `json:"error_count"`
	LastStatus *int   `json:"last_status,omitempty"`
}

func doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose the database, feeds and health counters",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(runDoctor)
		},
	}
}

func runDoctor(st *store.Store) error {
	path, _ := dbPath()
	feeds, err := st.ListFeeds()
	if err != nil {
		return err
	}
	articles, err := st.CountArticles()
	if err != nil {
		return err
	}
	unread, err := st.CountUnread()
	if err != nil {
		return err
	}
	flagged := unhealthyFeeds(feeds)

	if flagJSON {
		return printJSON(map[string]any{
			"db_path":         path,
			"feeds":           len(feeds),
			"articles":        articles,
			"unread":          unread,
			"error_threshold": errorThreshold,
			"unhealthy_feeds": flagged,
			"ok":              len(flagged) == 0,
		})
	}
	printDoctorText(path, len(feeds), articles, unread, flagged)
	return nil
}

func unhealthyFeeds(feeds []*store.Feed) []unhealthyFeed {
	var flagged []unhealthyFeed
	for _, f := range feeds {
		if f.ErrorCount >= errorThreshold {
			flagged = append(flagged, unhealthyFeed{f.ID, f.URL, f.ErrorCount, f.LastStatus})
		}
	}
	return flagged
}

func printDoctorText(path string, feeds, articles, unread int, flagged []unhealthyFeed) {
	fmt.Printf("database: %s\n", path)
	fmt.Printf("feeds:    %d\n", feeds)
	fmt.Printf("articles: %d (%d unread)\n", articles, unread)
	if len(flagged) == 0 {
		fmt.Printf("health:   OK — no feeds over %d consecutive errors\n", errorThreshold)
		return
	}
	fmt.Printf("health:   %d feed(s) with >= %d consecutive errors:\n", len(flagged), errorThreshold)
	for _, u := range flagged {
		fmt.Printf("  - #%d errors=%d %s\n", u.ID, u.ErrorCount, u.URL)
	}
}
