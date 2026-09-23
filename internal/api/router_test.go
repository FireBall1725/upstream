// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fireball1725/upstream/internal/bump"
	"github.com/fireball1725/upstream/internal/config"
	"github.com/fireball1725/upstream/internal/db"
	"github.com/fireball1725/upstream/internal/repository"
	"github.com/fireball1725/upstream/internal/scan"
)

func serve(t *testing.T, cfg *config.Config, path string) *httptest.ResponseRecorder {
	t.Helper()
	return do(t, cfg, http.MethodGet, path)
}

func do(t *testing.T, cfg *config.Config, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	return doBody(t, cfg, method, path, "")
}

func doBody(t *testing.T, cfg *config.Config, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	conn, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	repo := repository.New(conn)
	scans := scan.New(cfg, repo)
	rec := httptest.NewRecorder()
	NewRouter(cfg, repo, scans, bump.New(cfg, repo, scans, nil)).ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

func TestScanWithoutRepoIsAConflict(t *testing.T) {
	rec := do(t, &config.Config{}, http.MethodPost, "/api/scan")
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409", rec.Code)
	}
	var body errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Error == "" {
		t.Errorf("body %s", rec.Body.String())
	}
}

func TestLatestScanBeforeAnyScan(t *testing.T) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(serve(t, &config.Config{}, "/api/scan").Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if string(raw["state"]) != `"never"` || string(raw["apps"]) != "[]" {
		t.Errorf("state %s apps %s", raw["state"], raw["apps"])
	}
}

func TestUnknownAPIPathIsNotTheUI(t *testing.T) {
	rec := serve(t, &config.Config{}, "/api/nope")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404 so a client bug can't read index.html as JSON", rec.Code)
	}
}

func TestStatusListsProblemsAsAnEmptyArray(t *testing.T) {
	cfg := &config.Config{
		Repo: "https://github.com/o/r.git", Branch: "main", AppGlob: "apps/*/*", Schedule: "0 */6 * * *",
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
	// Repo, schedule, token and author are all missing.
	if len(body.Problems) != 4 || body.PRsReady {
		t.Errorf("got %d problems, want 4: %v", len(body.Problems), body.Problems)
	}
}

func TestPRsNeedConfig(t *testing.T) {
	rec := doBody(t, &config.Config{Repo: "https://github.com/o/r.git"}, http.MethodPost, "/api/prs", `{"apps":["apps/a/b"]}`)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "GITHUB_TOKEN") {
		t.Errorf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestSkipNeedsAllFields(t *testing.T) {
	rec := doBody(t, &config.Config{}, http.MethodPost, "/api/skips", `{"appDir":"apps/a/b"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status %d", rec.Code)
	}
}

func TestMetrics(t *testing.T) {
	rec := serve(t, &config.Config{}, "/metrics")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "upstream_apps 0") || !strings.Contains(rec.Body.String(), "upstream_open_prs 0") {
		t.Errorf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestStatusFlagsANonGitHubRepo(t *testing.T) {
	var body statusResponse
	_ = json.Unmarshal(serve(t, &config.Config{Repo: "https://gitea.local/o/r.git", Schedule: "0 * * * *"}, "/api/status").Body.Bytes(), &body)
	found := false
	for _, p := range body.Problems {
		if strings.Contains(p, "github.com") {
			found = true
		}
	}
	if !found {
		t.Errorf("problems %v", body.Problems)
	}
}
