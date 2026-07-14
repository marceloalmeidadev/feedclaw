package api

import (
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// embeddedUI is nil unless the binary is built with -tags embedui (see
// ui_embed.go), in which case it holds the generated SPA. Serving the UI from
// the same origin as the API is why the API needs no CORS.
var embeddedUI fs.FS

// serveUI serves the static UI for non-/api routes. Unknown paths fall back to
// the SPA shell (200.html, then index.html) so client-side routes like
// /articles/12 load the app instead of 404ing. When no UI is embedded it
// returns the JSON not-found response.
func (s *Server) serveUI(w http.ResponseWriter, r *http.Request) {
	if s.ui == nil {
		s.notFound(w, r)
		return
	}
	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if name == "" {
		name = "index.html"
	}
	if serveFile(w, r, s.ui, name) {
		return
	}
	for _, shell := range []string{"200.html", "index.html"} {
		if serveFile(w, r, s.ui, shell) {
			return
		}
	}
	s.notFound(w, r)
}

// serveFile writes fsys/name if it resolves to a regular file (following one
// level into directories via index.html), returning true when it responded.
func serveFile(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string) bool {
	f, err := fsys.Open(name)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return false
	}
	if info.IsDir() {
		return serveFile(w, r, fsys, path.Join(name, "index.html"))
	}
	rs, ok := f.(io.ReadSeeker)
	if !ok {
		return false
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), rs)
	return true
}
