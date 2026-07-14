//go:build embedui

// This file is only compiled for release builds (`-tags embedui`), which embed
// the generated Nuxt SPA. The Makefile copies ui/.output/public into dist/
// before building. Plain `go build`/`go test` omit the tag, so `dist/` is not
// required for development or CI.
package api

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

func init() {
	if sub, err := fs.Sub(distFS, "dist"); err == nil {
		embeddedUI = sub
	}
}
