package api

import (
	"net/http"
	"strconv"

	"github.com/marceloalmeidadev/feedclaw/internal/fetch"
	"github.com/marceloalmeidadev/feedclaw/internal/opml"
	"github.com/marceloalmeidadev/feedclaw/internal/readability"
	"github.com/marceloalmeidadev/feedclaw/internal/store"
)

// --- feeds -----------------------------------------------------------------

func (s *Server) listFeeds(w http.ResponseWriter, r *http.Request) {
	feeds, err := s.store.ListFeeds()
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"feeds": feeds})
}

func (s *Server) addFeed(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL      string `json:"url"`
		Category string `json:"category"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if body.URL == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "url is required")
		return
	}
	feed, created, err := s.store.AddFeed(body.URL, "", "", body.Category)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, map[string]any{"feed": feed, "created": created})
}

// importFeeds imports an uploaded OPML document (raw request body). The OPML
// parser is XXE-safe; the body is size-limited to guard against huge uploads.
func (s *Server) importFeeds(w http.ResponseWriter, r *http.Request) {
	feeds, err := opml.Parse(http.MaxBytesReader(w, r.Body, 16<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_opml", err.Error())
		return
	}
	var added, existing int
	for _, f := range feeds {
		_, created, err := s.store.AddFeed(f.XMLURL, f.Title, f.HTMLURL, f.Category)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		if created {
			added++
		} else {
			existing++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"total": len(feeds), "added": added, "existing": existing})
}

func (s *Server) deleteFeed(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	if err := s.store.RemoveFeedByID(id); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"removed": id})
}

// --- fetch -----------------------------------------------------------------

func (s *Server) runFetch(w http.ResponseWriter, r *http.Request) {
	f := fetch.New(s.store, s.fetchCfg)
	results, err := f.Run(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fetch_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "done", "results": results})
}

// --- articles --------------------------------------------------------------

func (s *Server) listArticles(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := store.ArticleFilter{
		Status:   q.Get("status"),
		Category: q.Get("category"),
		Query:    q.Get("q"),
		ThemeID:  atoi64(q.Get("theme")),
		Page:     atoi(q.Get("page")),
		PerPage:  atoi(q.Get("per_page")),
	}
	articles, total, err := s.store.ListArticles(filter)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"articles": articles,
		"total":    total,
		"page":     page,
	})
}

func (s *Server) getArticle(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	article, err := s.store.ArticleByID(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, article)
}

func (s *Server) fullArticle(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	article, err := s.store.ArticleByID(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	client, _ := fetch.Client(s.fetchCfg)
	html, err := readability.Extract(r.Context(), client, article.URL, s.fetchCfg.MaxArticleBytes)
	if err != nil {
		writeError(w, http.StatusBadGateway, "extract_failed", err.Error())
		return
	}
	if err := s.store.SetFullContent(id, html); err != nil {
		writeStoreError(w, err)
		return
	}
	article.FullContent = html
	writeJSON(w, http.StatusOK, article)
}

func (s *Server) patchRead(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs  []int64 `json:"ids"`
		Read bool    `json:"read"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	n, err := s.store.SetRead(body.IDs, body.Read)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"affected": n})
}

func (s *Server) patchStar(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs     []int64 `json:"ids"`
		Starred bool    `json:"starred"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	n, err := s.store.SetStarred(body.IDs, body.Starred)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"affected": n})
}

// --- digests ---------------------------------------------------------------

func (s *Server) getDigests(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		latest, err := s.store.LatestDigestDate()
		if err != nil {
			writeStoreError(w, err)
			return
		}
		date = latest
	}
	digest, err := s.store.GetDigest(date)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, digest)
}

func (s *Server) themeArticles(w http.ResponseWriter, r *http.Request) {
	themeID, ok := pathInt64(w, r, "themeId")
	if !ok {
		return
	}
	articles, err := s.store.ThemeArticles(themeID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"articles": articles})
}

// --- stats -----------------------------------------------------------------

func (s *Server) getStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.Stats()
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// --- helpers ---------------------------------------------------------------

func pathInt64(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid "+name)
		return 0, false
	}
	return id, true
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func atoi64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}
