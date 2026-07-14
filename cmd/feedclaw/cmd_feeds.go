package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/marceloalmeidadev/feedclaw/internal/fetch"
	"github.com/marceloalmeidadev/feedclaw/internal/opml"
	"github.com/marceloalmeidadev/feedclaw/internal/store"
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
			return withStore(func(st *store.Store) error {
				feeds, err := st.ListFeeds()
				if err != nil {
					return err
				}
				if flagJSON {
					return printJSON(feeds)
				}
				printFeedTable(feeds)
				return nil
			})
		},
	}
}

func printFeedTable(feeds []*store.Feed) {
	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tTITLE\tCATEGORY\tERRORS\tURL")
	for _, f := range feeds {
		_, _ = fmt.Fprintf(tw, "%d\t%s\t%s\t%d\t%s\n", f.ID, truncate(f.Title, 40), f.Category, f.ErrorCount, f.URL)
	}
	_ = tw.Flush()
}

func feedsAddCmd() *cobra.Command {
	var category string
	cmd := &cobra.Command{
		Use:   "add <url>",
		Short: "Add a feed by URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(func(st *store.Store) error {
				feed, created, err := st.AddFeed(args[0], "", "", category)
				if err != nil {
					return err
				}
				if flagJSON {
					return printJSON(map[string]any{"feed": feed, "created": created})
				}
				verb := "already present"
				if created {
					verb = "added feed"
				}
				fmt.Printf("%s #%d: %s\n", verb, feed.ID, feed.URL)
				return nil
			})
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
			return withStore(func(st *store.Store) error {
				if err := st.RemoveFeed(args[0]); err != nil {
					return err
				}
				if flagJSON {
					return printJSON(map[string]any{"removed": args[0]})
				}
				fmt.Printf("removed feed: %s\n", args[0])
				return nil
			})
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
			return runImport(opmlPath)
		},
	}
	cmd.Flags().StringVar(&opmlPath, "opml", "", "path or URL to an OPML file")
	return cmd
}

func runImport(opmlPath string) error {
	feeds, err := loadOPML(opmlPath)
	if err != nil {
		return err
	}
	return withStore(func(st *store.Store) error {
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
		if flagJSON {
			return printJSON(map[string]any{"total": len(feeds), "added": added, "existing": existing})
		}
		fmt.Printf("imported %d feeds (%d new, %d already present)\n", len(feeds), added, existing)
		return nil
	})
}

// maxOPMLBytes caps a remotely-fetched OPML document (a Feedly export with
// hundreds of feeds is tens of KB); guards against a hostile oversized body.
const maxOPMLBytes = 4 << 20

// loadOPML reads an OPML document from a local path or an http(s) URL. Remote
// fetches go through the SSRF-guarded client (blocks private/loopback targets,
// revalidates redirect hops) and are byte-limited — never a raw http.Client.
func loadOPML(src string) ([]opml.Feed, error) {
	if u, err := url.Parse(src); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		client, _ := fetch.Client(fetch.Config{})
		resp, err := client.Get(src)
		if err != nil {
			return nil, fmt.Errorf("fetch opml: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("fetch opml: status %d", resp.StatusCode)
		}
		return opml.Parse(io.LimitReader(resp.Body, maxOPMLBytes))
	}
	file, err := os.Open(src)
	if err != nil {
		return nil, fmt.Errorf("open opml: %w", err)
	}
	defer func() { _ = file.Close() }()
	return opml.Parse(file)
}

// truncate shortens s to at most n runes (not bytes) so multibyte characters —
// common in pt-BR titles — are never cut mid-character into invalid UTF-8.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n-1])) + "…"
}
