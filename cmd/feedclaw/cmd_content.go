package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/marceloalmeidadev/feedclaw/internal/fetch"
	"github.com/marceloalmeidadev/feedclaw/internal/readability"
	"github.com/spf13/cobra"
)

func fullCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "full <article-id>",
		Short: "Return an article's full content, extracting and caching if absent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid article id %q", args[0])
			}
			st, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()

			article, err := st.ArticleByID(id)
			if err != nil {
				return err
			}

			if article.FullContent == "" || force {
				client, _ := fetch.Client(fetch.Config{})
				html, err := readability.Extract(context.Background(), client, article.URL, 0)
				if err != nil {
					return fmt.Errorf("extract full content: %w", err)
				}
				if err := st.SetFullContent(id, html); err != nil {
					return err
				}
				article.FullContent = html
			}

			if flagJSON {
				return printJSON(article)
			}
			fmt.Printf("# %s\n%s\n\n%s\n", article.Title, article.URL, article.FullContent)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "re-extract even if already cached")
	return cmd
}

func searchCmd() *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search across articles (FTS5)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			st, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()

			articles, err := st.Search(query, limit)
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
	cmd.Flags().IntVar(&limit, "limit", 50, "maximum number of results")
	return cmd
}
