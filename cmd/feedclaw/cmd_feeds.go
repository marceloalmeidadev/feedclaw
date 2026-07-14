package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/marceloalmeidadev/feedclaw/internal/opml"
	"github.com/spf13/cobra"
)

func feedsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "feeds", Short: "Manage feed subscriptions"}
	cmd.AddCommand(feedsListCmd(), feedsAddCmd(), feedsRemoveCmd())
	return cmd
}

func feedsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all feeds",
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()
			feeds, err := st.ListFeeds()
			if err != nil {
				return err
			}
			if flagJSON {
				return printJSON(feeds)
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			_, _ = fmt.Fprintln(tw, "ID\tTITLE\tCATEGORY\tERRORS\tURL")
			for _, f := range feeds {
				_, _ = fmt.Fprintf(tw, "%d\t%s\t%s\t%d\t%s\n", f.ID, truncate(f.Title, 40), f.Category, f.ErrorCount, f.URL)
			}
			return tw.Flush()
		},
	}
}

func feedsAddCmd() *cobra.Command {
	var category string
	cmd := &cobra.Command{
		Use:   "add <url>",
		Short: "Add a feed by URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()
			feed, created, err := st.AddFeed(args[0], "", "", category)
			if err != nil {
				return err
			}
			if flagJSON {
				return printJSON(map[string]any{"feed": feed, "created": created})
			}
			if created {
				fmt.Printf("added feed #%d: %s\n", feed.ID, feed.URL)
			} else {
				fmt.Printf("feed already present #%d: %s\n", feed.ID, feed.URL)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "category/folder for the feed")
	return cmd
}

func feedsRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <url>",
		Short: "Remove a feed and its articles",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()
			if err := st.RemoveFeed(args[0]); err != nil {
				return err
			}
			if flagJSON {
				return printJSON(map[string]any{"removed": args[0]})
			}
			fmt.Printf("removed feed: %s\n", args[0])
			return nil
		},
	}
}

func importCmd() *cobra.Command {
	var opmlPath string
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import feeds from an OPML file or URL",
		RunE: func(cmd *cobra.Command, args []string) error {
			if opmlPath == "" {
				return fmt.Errorf("--opml <path|url> is required")
			}
			feeds, err := loadOPML(opmlPath)
			if err != nil {
				return err
			}
			st, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()

			var added, existing int
			for _, f := range feeds {
				_, created, err := st.AddFeed(f.XMLURL, f.Title, f.HTMLURL, f.Category)
				if err != nil {
					return fmt.Errorf("add %s: %w", f.XMLURL, err)
				}
				if created {
					added++
				} else {
					existing++
				}
			}
			result := map[string]any{"total": len(feeds), "added": added, "existing": existing}
			if flagJSON {
				return printJSON(result)
			}
			fmt.Printf("imported %d feeds (%d new, %d already present)\n", len(feeds), added, existing)
			return nil
		},
	}
	cmd.Flags().StringVar(&opmlPath, "opml", "", "path or URL to an OPML file")
	return cmd
}

// loadOPML reads an OPML document from a local path or an http(s) URL.
func loadOPML(src string) ([]opml.Feed, error) {
	if u, err := url.Parse(src); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get(src)
		if err != nil {
			return nil, fmt.Errorf("fetch opml: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("fetch opml: status %d", resp.StatusCode)
		}
		return opml.Parse(resp.Body)
	}
	file, err := os.Open(src)
	if err != nil {
		return nil, fmt.Errorf("open opml: %w", err)
	}
	defer func() { _ = file.Close() }()
	return opml.Parse(file)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n-1]) + "…"
}
