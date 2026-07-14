package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// errorThreshold is the consecutive-failure count at which a feed is flagged
// (never auto-removed) in doctor output and the UI.
const errorThreshold = 10

func doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose the database, feeds and health counters",
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()

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

			type unhealthy struct {
				ID         int64  `json:"id"`
				URL        string `json:"url"`
				ErrorCount int    `json:"error_count"`
				LastStatus *int   `json:"last_status,omitempty"`
			}
			var flagged []unhealthy
			for _, f := range feeds {
				if f.ErrorCount >= errorThreshold {
					flagged = append(flagged, unhealthy{f.ID, f.URL, f.ErrorCount, f.LastStatus})
				}
			}

			report := map[string]any{
				"db_path":         path,
				"feeds":           len(feeds),
				"articles":        articles,
				"unread":          unread,
				"error_threshold": errorThreshold,
				"unhealthy_feeds": flagged,
				"ok":              len(flagged) == 0,
			}
			if flagJSON {
				return printJSON(report)
			}

			fmt.Printf("database: %s\n", path)
			fmt.Printf("feeds:    %d\n", len(feeds))
			fmt.Printf("articles: %d (%d unread)\n", articles, unread)
			if len(flagged) == 0 {
				fmt.Printf("health:   OK — no feeds over %d consecutive errors\n", errorThreshold)
			} else {
				fmt.Printf("health:   %d feed(s) with >= %d consecutive errors:\n", len(flagged), errorThreshold)
				for _, u := range flagged {
					fmt.Printf("  - #%d errors=%d %s\n", u.ID, u.ErrorCount, u.URL)
				}
			}
			return nil
		},
	}
}
