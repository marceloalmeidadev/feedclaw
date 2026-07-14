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
			return runUnread(since, category, limit)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "only articles newer than this (e.g. 24h, 7d)")
	cmd.Flags().StringVar(&category, "category", "", "filter by category")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum number of articles (0 = no limit)")
	return cmd
}

func runUnread(since, category string, limit int) error {
	filter := store.UnreadFilter{Category: category, Limit: limit}
	if since != "" {
		d, err := parseDuration(since)
		if err != nil {
			return err
		}
		filter.Since = d
	}
	return withStore(func(st *store.Store) error {
		articles, err := st.ListUnread(filter)
		if err != nil {
			return err
		}
		if flagJSON {
			return printJSON(articles)
		}
		printArticleTable(articles)
		return nil
	})
}

func markCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "mark", Short: "Change read state of articles"}
	cmd.AddCommand(markReadCmd(), markUnreadCmd())
	return cmd
}

func markReadCmd() *cobra.Command {
	var (
		olderThan  string
		allInTheme int64
	)
	cmd := &cobra.Command{
		Use:   "read [article-id...]",
		Short: "Mark articles as read (by id, --older-than or --all-in-theme)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMarkRead(args, olderThan, allInTheme)
		},
	}
	cmd.Flags().StringVar(&olderThan, "older-than", "", "mark all unread articles older than this (e.g. 7d)")
	cmd.Flags().Int64Var(&allInTheme, "all-in-theme", 0, "mark every unread article in this theme id as read")
	return cmd
}

func runMarkRead(args []string, olderThan string, allInTheme int64) error {
	if olderThan == "" && len(args) == 0 && allInTheme == 0 {
		return fmt.Errorf("provide article ids, --older-than or --all-in-theme")
	}
	ids, err := parseIDs(args)
	if err != nil {
		return err
	}
	age, err := optionalDuration(olderThan)
	if err != nil {
		return err
	}
	return withStore(func(st *store.Store) error {
		return applyMarkRead(st, ids, age, allInTheme)
	})
}

// optionalDuration parses a duration flag that may be empty (meaning "unset").
func optionalDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	return parseDuration(s)
}

// applyMarkRead marks the theme, the older-than window and the explicit ids as
// read, summing how many rows were affected.
func applyMarkRead(st *store.Store, ids []int64, age time.Duration, themeID int64) error {
	var affected int64
	if themeID > 0 {
		n, err := st.MarkReadAllInTheme(themeID)
		if err != nil {
			return err
		}
		affected += n
	}
	if age > 0 {
		n, err := st.MarkReadOlderThan(age)
		if err != nil {
			return err
		}
		affected += n
	}
	if len(ids) > 0 {
		n, err := st.SetRead(ids, true)
		if err != nil {
			return err
		}
		affected += n
	}
	return reportAffected(affected, "marked read")
}

func markUnreadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unread <article-id...>",
		Short: "Mark articles as unread",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetRead(args, false, "marked unread")
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
			return runSetStar(args, starred, verb)
		},
	}
}

func runSetRead(args []string, read bool, label string) error {
	ids, err := parseIDs(args)
	if err != nil {
		return err
	}
	return withStore(func(st *store.Store) error {
		n, err := st.SetRead(ids, read)
		if err != nil {
			return err
		}
		return reportAffected(n, label)
	})
}

func runSetStar(args []string, starred bool, label string) error {
	ids, err := parseIDs(args)
	if err != nil {
		return err
	}
	return withStore(func(st *store.Store) error {
		n, err := st.SetStarred(ids, starred)
		if err != nil {
			return err
		}
		return reportAffected(n, label)
	})
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
		_, _ = fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n",
			a.ID, articleState(a), truncate(a.FeedTitle, 20), relTime(a.PublishedAt), truncate(a.Title, 60))
	}
	_ = tw.Flush()
}

func articleState(a *store.Article) string {
	state := "unread"
	if a.ReadAt != nil {
		state = "read"
	}
	if a.Starred {
		state += "*"
	}
	return state
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
