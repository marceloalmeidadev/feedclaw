package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/marceloalmeidadev/feedclaw/internal/api"
	"github.com/marceloalmeidadev/feedclaw/internal/fetch"
	"github.com/spf13/cobra"
)

func serveCmd() *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the local REST API (and UI) on 127.0.0.1",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServe(port)
		},
	}
	cmd.Flags().IntVar(&port, "port", 8484, "TCP port to listen on (127.0.0.1 only)")
	return cmd
}

func runServe(port int) error {
	// serve holds the store open for the process lifetime instead of the usual
	// open-per-command pattern.
	st, err := openStore()
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()

	// Bind exclusively to loopback — never 0.0.0.0.
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           api.New(st, fetch.Config{}).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("FeedClaw API listening on http://%s", addr)
	return srv.ListenAndServe()
}
