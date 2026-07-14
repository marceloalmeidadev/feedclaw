package main

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/marceloalmeidadev/feedclaw/internal/store"
	"github.com/spf13/cobra"
)

func unreadCmd() *cobra.Command {
	var (
		since    string
		category string
		limit    int
	)
	cmd := &cobra.Command{
		Use:   "unread",
		Short: "List unread articles (used to export for clustering)",
		RunE: func(cmd *cobra.Command, args []string) error {
			filter := store.UnreadFilter{Category: category, Limit: limit}
			if since != "" {
				d, err := parseDuration(since)
				if err != nil {
					return err
				}
				filter.Since = d
			}
			st, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()

			articles, err := st.ListUnread(filter)
			if err != nil {
				return err
			}
			if flagJSON {
				return printJSON(articles)
			}
			printArticleTable(articles)
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "only articles newer than this (e.g. 24h, 7d)")
	cmd.Flags().StringVar(&category, "category", "", "filter by category")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum number of articles (0 = no limit)")
	return cmd
}

func markCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "mark", Short: "Change read state of articles"}
	cmd.AddCommand(markReadCmd(), markUnreadCmd())
	return cmd
}

func markReadCmd() *cobra.Command {
	var olderThan string
	cmd := &cobra.Command{
		Use:   "read [article-id...]",
		Short: "Mark articles as read (by id, or --older-than)",
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()

			var affected int64
			if olderThan != "" {
				d, err := parseDuration(olderThan)
				if err != nil {
					return err
				}
				affected, err = st.MarkReadOlderThan(d)
				if err != nil {
					return err
				}
			}
			if len(args) > 0 {
				ids, err := parseIDs(args)
				if err != nil {
					return err
				}
				n, err := st.SetRead(ids, true)
				if err != nil {
					return err
				}
				affected += n
			}
			if olderThan == "" && len(args) == 0 {
				return fmt.Errorf("provide article ids or --older-than")
			}
			return reportAffected(affected, "marked read")
		},
	}
	cmd.Flags().StringVar(&olderThan, "older-than", "", "mark all unread articles older than this (e.g. 7d)")
	return cmd
}

func markUnreadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unread <article-id...>",
		Short: "Mark articles as unread",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ids, err := parseIDs(args)
			if err != nil {
				return err
			}
			st, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()
			n, err := st.SetRead(ids, false)
			if err != nil {
				return err
			}
			return reportAffected(n, "marked unread")
		},
	}
}

func starCmd(starred bool) *cobra.Command {
	use, verb := "star", "starred"
	if !starred {
		use, verb = "unstar", "unstarred"
	}
	return &cobra.Command{
		Use:   use + " <article-id...>",
		Short: fmt.Sprintf("Mark articles as %s (read-it-later)", verb),
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ids, err := parseIDs(args)
			if err != nil {
				return err
			}
			st, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()
			n, err := st.SetStarred(ids, starred)
			if err != nil {
				return err
			}
			return reportAffected(n, verb)
		},
	}
}

// reportAffected prints a small JSON or text summary of a mutation.
func reportAffected(n int64, action string) error {
	if flagJSON {
		return printJSON(map[string]any{"affected": n, "action": action})
	}
	fmt.Printf("%s: %d article(s)\n", action, n)
	return nil
}

// printArticleTable renders a compact triage-friendly table.
func printArticleTable(articles []*store.Article) {
	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tSTATE\tFEED\tPUBLISHED\tTITLE")
	for _, a := range articles {
		state := "unread"
		if a.ReadAt != nil {
			state = "read"
		}
		if a.Starred {
			state += "*"
		}
		_, _ = fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n",
			a.ID, state, truncate(a.FeedTitle, 20), relTime(a.PublishedAt), truncate(a.Title, 60))
	}
	_ = tw.Flush()
}

// relTime formats a timestamp as a short relative age (e.g. "3h", "2d").
func relTime(t *time.Time) string {
	if t == nil {
		return "-"
	}
	d := time.Since(*t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
