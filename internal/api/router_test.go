// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fireball1725/upstream/internal/config"
)

func serve(t *testing.T, cfg *config.Config, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	NewRouter(cfg).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestUnknownAPIPathIsNotTheUI(t *testing.T) {
	rec := serve(t, &config.Config{}, "/api/nope")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404 so a client bug can't read index.html as JSON", rec.Code)
	}
}

func TestStatusListsProblemsAsAnEmptyArray(t *testing.T) {
	cfg := &config.Config{
		Repo: "https://example.com/r.git", Branch: "main", AppGlob: "apps/*/*",
		GitHubToken: "x", GitAuthorName: "a", GitAuthorEmail: "a@example.com",
	}
	rec := serve(t, cfg, "/api/status")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	// A nil slice marshals to null and the web client crashes on .length.
	if got := string(raw["problems"]); got != "[]" {
		t.Errorf("problems = %s, want []", got)
	}
}

func TestStatusReportsMissingRepo(t *testing.T) {
	var body statusResponse
	if err := json.Unmarshal(serve(t, &config.Config{}, "/api/status").Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Problems) != 3 {
		t.Errorf("got %d problems, want 3: %v", len(body.Problems), body.Problems)
	}
}
