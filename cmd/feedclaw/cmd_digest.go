package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/marceloalmeidadev/feedclaw/internal/digest"
	"github.com/marceloalmeidadev/feedclaw/internal/store"
	"github.com/spf13/cobra"
)

func digestCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "digest", Short: "Daily grouped digest"}
	cmd.AddCommand(digestSaveCmd(), digestShowCmd())
	return cmd
}

// today returns the current date in the machine's LOCAL timezone. Using UTC
// would roll the date over in the evening for users west of Greenwich (e.g.
// UTC−3), so "hoje" and the default digest date must follow local time. The
// SKILL.md accordingly uses `date +%F` (not `date -u`).
func today() string { return time.Now().Local().Format("2006-01-02") }

func digestSaveCmd() *cobra.Command {
	var (
		date  string
		input string
	)
	cmd := &cobra.Command{
		Use:   "save",
		Short: "Persist the agent-grouped digest (residual 'Outros' theme added automatically)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if date == "" {
				date = today()
			}
			return runDigestSave(date, input)
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "digest date YYYY-MM-DD (default: today, UTC)")
	cmd.Flags().StringVar(&input, "input", "-", "path to the digest JSON, or '-' for stdin")
	return cmd
}

func runDigestSave(date, input string) error {
	reader := os.Stdin
	if input != "-" {
		f, err := os.Open(input)
		if err != nil {
			return fmt.Errorf("open input: %w", err)
		}
		defer func() { _ = f.Close() }()
		reader = f
	}
	in, err := digest.ParseInput(reader)
	if err != nil {
		return err
	}
	return withStore(func(st *store.Store) error {
		d, err := digest.Save(st, date, in)
		if err != nil {
			return err
		}
		if flagJSON {
			return printJSON(d)
		}
		printDigest(d)
		return nil
	})
}

func digestShowCmd() *cobra.Command {
	var date string
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show today's digest (or a given/most-recent date)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDigestShow(date)
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "digest date YYYY-MM-DD (default: today, else most recent)")
	return cmd
}

func runDigestShow(date string) error {
	return withStore(func(st *store.Store) error {
		d, err := resolveDigest(st, date)
		if err != nil {
			return err
		}
		if flagJSON {
			return printJSON(d)
		}
		printDigest(d)
		return nil
	})
}

// resolveDigest picks the digest to show: the given date, else today, else the
// most recent one.
func resolveDigest(st *store.Store, date string) (*store.Digest, error) {
	if date != "" {
		return st.GetDigest(date)
	}
	d, err := st.GetDigest(today())
	if err == nil {
		return d, nil
	}
	if err != store.ErrNotFound {
		return nil, err
	}
	latest, err := st.LatestDigestDate()
	if err != nil {
		return nil, err
	}
	return st.GetDigest(latest)
}

func themeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "theme <theme-id>",
		Short: "List all articles in a digest theme",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid theme id %q", args[0])
			}
			return runTheme(id)
		},
	}
}

func runTheme(id int64) error {
	return withStore(func(st *store.Store) error {
		theme, err := st.ThemeByID(id)
		if err != nil {
			return err
		}
		articles, err := st.ThemeArticles(id)
		if err != nil {
			return err
		}
		theme.Articles = articles
		theme.ArticleCount = len(articles)
		if flagJSON {
			return printJSON(theme)
		}
		fmt.Printf("Tema #%d — %s\n%s\n\n", theme.ID, theme.Name, theme.Summary)
		printArticleTable(articles)
		return nil
	})
}

// printDigest renders a digest as a themed overview (pt-BR).
func printDigest(d *store.Digest) {
	header := "Notícias de " + d.Date
	if d.Date == today() {
		header = "Notícias para hoje (" + d.Date + ")"
	}
	fmt.Printf("%s — gerado %s\n\n", header, d.GeneratedAt.Format("02/01/2006 15:04"))
	for _, t := range d.Themes {
		fmt.Printf("[%d] %s  (tema #%d, %d artigo(s))\n", t.Position, t.Name, t.ID, t.ArticleCount)
		if t.Summary != "" {
			fmt.Printf("    %s\n", t.Summary)
		}
	}
	fmt.Printf("\nUse 'feedclaw theme <id>' para ver os artigos de um tema.\n")
}
