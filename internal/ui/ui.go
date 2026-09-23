// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

// Package ui serves the React build that Vite writes into dist/.
package ui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// dist holds the Vite output; .gitkeep keeps the directory present so a Go-only build still compiles.
//
//go:embed all:dist
var dist embed.FS

const notBuilt = `<!doctype html><meta charset="utf-8"><title>Upstream</title>
<p>The web UI isn't in this build. Run <code>npm run build</code> in <code>web/</code>, then rebuild the binary.</p>`

// Handler serves static files and falls back to index.html so client-side routes survive a reload.
func Handler() http.Handler {
	root, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return handlerFor(root)
}

func handlerFor(root fs.FS) http.Handler {
	index, err := fs.ReadFile(root, "index.html")
	files := http.FileServerFS(root)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "" && name != "index.html" {
			if f, statErr := fs.Stat(root, name); statErr == nil && !f.IsDir() {
				if strings.HasPrefix(name, "assets/") {
					// Vite hashes asset file names, so they never change under the same URL.
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		if err != nil {
			w.Write([]byte(notBuilt))
			return
		}
		w.Write(index)
	})
}
