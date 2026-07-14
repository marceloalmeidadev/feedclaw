// Package api serves the FeedClaw REST API (and, from Phase 6, the embedded
// UI) over 127.0.0.1 only. The API is the same source of truth the agent uses
// via the CLI.
package api

import (
	"log"
	"net/http"

	"github.com/marceloalmeidadev/feedclaw/internal/fetch"
	"github.com/marceloalmeidadev/feedclaw/internal/store"
)

// Server holds the dependencies shared by the HTTP handlers.
type Server struct {
	store    *store.Store
	fetchCfg fetch.Config
}

// New builds a Server bound to a store and fetcher config.
func New(st *store.Store, cfg fetch.Config) *Server {
	return &Server{store: st, fetchCfg: cfg}
}

// Handler returns the fully wired HTTP handler (routes + middleware).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/feeds", s.listFeeds)
	mux.HandleFunc("POST /api/feeds", s.addFeed)
	mux.HandleFunc("POST /api/feeds/import", s.importFeeds)
	mux.HandleFunc("DELETE /api/feeds/{id}", s.deleteFeed)

	mux.HandleFunc("POST /api/fetch", s.runFetch)

	mux.HandleFunc("GET /api/articles", s.listArticles)
	mux.HandleFunc("GET /api/articles/{id}", s.getArticle)
	mux.HandleFunc("POST /api/articles/{id}/full", s.fullArticle)
	mux.HandleFunc("PATCH /api/articles/read", s.patchRead)
	mux.HandleFunc("PATCH /api/articles/star", s.patchStar)

	mux.HandleFunc("GET /api/digests", s.getDigests)
	mux.HandleFunc("GET /api/digests/{date}/themes/{themeId}/articles", s.themeArticles)

	mux.HandleFunc("GET /api/stats", s.getStats)

	mux.HandleFunc("/", s.notFound)

	return recoverMiddleware(authMiddleware(mux))
}

// notFound handles unmatched routes (the UI takes over "/" in Phase 6).
func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", "no such route: "+r.URL.Path)
}

// authMiddleware is a pass-through today (the API binds to localhost only) but
// gives us a single place to require a token later.
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

// recoverMiddleware turns a handler panic into a 500 JSON error instead of a
// dropped connection.
func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic serving %s %s: %v", r.Method, r.URL.Path, rec)
				writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
