package main

import (
	"context"
	"fmt"

	"github.com/marceloalmeidadev/feedclaw/internal/fetch"
	"github.com/marceloalmeidadev/feedclaw/internal/store"
	"github.com/spf13/cobra"
)

func fetchCmd() *cobra.Command {
	var (
		feedURL string
		workers int
	)
	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch feeds concurrently (conditional requests, SSRF guarded)",
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()

			var feeds []*store.Feed
			if feedURL != "" {
				f, err := st.FeedByURL(feedURL)
				if err != nil {
					return err
				}
				feeds = []*store.Feed{f}
			}

			f := fetch.New(st, fetch.Config{Workers: workers})
			results, err := f.Run(context.Background(), feeds)
			if err != nil {
				return err
			}

			if flagJSON {
				return printJSON(results)
			}
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
			return nil
		},
	}
	cmd.Flags().StringVar(&feedURL, "feed", "", "fetch only this feed URL")
	cmd.Flags().IntVar(&workers, "workers", 8, "number of concurrent workers")
	return cmd
}
