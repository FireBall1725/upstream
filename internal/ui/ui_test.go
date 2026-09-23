// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestHandler(t *testing.T) {
	built := fstest.MapFS{
		"index.html":         {Data: []byte("<html>app</html>")},
		"assets/index-a1.js": {Data: []byte("console.log(1)")},
	}

	tests := []struct {
		name      string
		root      fstest.MapFS
		path      string
		wantBody  string
		wantCache string
	}{
		{"root serves index", built, "/", "<html>app</html>", "no-cache"},
		{"client route falls back to index", built, "/apps/sonarr", "<html>app</html>", "no-cache"},
		{"hashed asset is cached for a year", built, "/assets/index-a1.js", "console.log(1)", "public, max-age=31536000, immutable"},
		{"no build says how to make one", fstest.MapFS{".gitkeep": {}}, "/", "npm run build", "no-cache"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handlerFor(tt.root).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body %q, want it to contain %q", rec.Body.String(), tt.wantBody)
			}
			if got := rec.Header().Get("Cache-Control"); got != tt.wantCache {
				t.Errorf("Cache-Control %q, want %q", got, tt.wantCache)
			}
		})
	}
}
