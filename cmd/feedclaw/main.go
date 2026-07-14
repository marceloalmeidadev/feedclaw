// Command feedclaw is the FeedClaw engine CLI: it fetches feeds, tracks read
// state in SQLite and serves the local API/UI. It is the single contract shared
// by the OpenClaw agent (via --json) and the web UI (via the HTTP API).
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/marceloalmeidadev/feedclaw/internal/fetch"
	"github.com/marceloalmeidadev/feedclaw/internal/store"
	"github.com/spf13/cobra"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

// global flags shared by every subcommand.
var (
	flagDB   string
	flagJSON bool
)

func main() {
	fetch.Version = version
	err := rootCmd().Execute()
	if err == nil {
		return
	}
	// An exitError carries a semantic process code (see exit.go). A nil inner
	// error is a clean non-zero exit (e.g. "nothing new") — no "error:" line.
	var ee *exitError
	if errors.As(err, &ee) {
		if ee.err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "error:", ee.err)
		}
		os.Exit(ee.code)
	}
	_, _ = fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "feedclaw",
		Short:         "Local RSS triage engine for OpenClaw",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&flagDB, "db", "", "path to the SQLite database (default: XDG config dir, e.g. ~/.config/feedclaw)")
	root.PersistentFlags().BoolVar(&flagJSON, "json", false, "structured JSON output")

	root.AddCommand(
		importCmd(),
		feedsCmd(),
		fetchCmd(),
		unreadCmd(),
		markCmd(),
		starCmd(true),
		starCmd(false),
		fullCmd(),
		searchCmd(),
		digestCmd(),
		themeCmd(),
		serveCmd(),
		doctorCmd(),
	)
	return root
}

// openStore resolves the database path and opens the store, creating parent
// directories as needed.
func openStore() (*store.Store, error) {
	path, err := dbPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	return store.Open(path)
}

// dbPath returns the effective database path, honoring --db then FEEDCLAW_DB
// then the XDG config directory (os.UserConfigDir, e.g. ~/.config).
func dbPath() (string, error) {
	if flagDB != "" {
		return flagDB, nil
	}
	if env := os.Getenv("FEEDCLAW_DB"); env != "" {
		return env, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config dir: %w", err)
	}
	return filepath.Join(dir, "feedclaw", "feedclaw.db"), nil
}

// printJSON writes v as indented JSON to stdout.
func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
