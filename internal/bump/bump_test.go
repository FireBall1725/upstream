// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

package bump

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fireball1725/upstream/internal/config"
	"github.com/fireball1725/upstream/internal/db"
	"github.com/fireball1725/upstream/internal/inventory"
	"github.com/fireball1725/upstream/internal/models"
	"github.com/fireball1725/upstream/internal/repository"
	"github.com/fireball1725/upstream/internal/scan"
)

type fakeScanner struct {
	result  scan.Result
	started int
}

func (f *fakeScanner) Latest() scan.Result                 { return f.result }
func (f *fakeScanner) Start(context.Context) (bool, error) { f.started++; return true, nil }

func sh(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=seed", "GIT_AUTHOR_EMAIL=seed@example.com", "GIT_COMMITTER_NAME=seed", "GIT_COMMITTER_EMAIL=seed@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out)
	}
	return string(out)
}

func write(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// fakeGitHub records calls and answers like the pulls API.
type fakeGitHub struct {
	mu    sync.Mutex
	calls []string
	body  map[string]any
}

func (f *fakeGitHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, r.Method+" "+r.URL.Path)
	if r.Header.Get("Authorization") != "Bearer tok" {
		http.Error(w, `{"message":"Bad credentials"}`, http.StatusUnauthorized)
		return
	}
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/repos/o/r/pulls":
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &f.body)
		_, _ = w.Write([]byte(`{"number":7,"html_url":"https://github.com/o/r/pull/7","state":"open"}`))
	case r.Method == http.MethodPut && r.URL.Path == "/repos/o/r/pulls/7/merge":
		_, _ = w.Write([]byte(`{"merged":true}`))
	case r.Method == http.MethodGet && r.URL.Path == "/repos/o/r/pulls/7":
		_, _ = w.Write([]byte(`{"number":7,"state":"closed","merged":true}`))
	default:
		w.WriteHeader(http.StatusUnprocessableEntity)
	}
}

type fixture struct {
	origin  string
	svc     *Service
	scanner *fakeScanner
	gh      *fakeGitHub
	repo    *repository.Repo
	helm    bool
}

func setup(t *testing.T) *fixture {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	_, helmErr := exec.LookPath("helm")
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	seed := filepath.Join(root, "seed")
	sh(t, root, "git", "init", "-q", "--bare", "-b", "main", origin)
	sh(t, root, "git", "init", "-q", "-b", "main", seed)

	files := map[string]string{
		"apps/app-spoolman/spoolman/Chart.yaml":  "apiVersion: v2\nname: spoolman\nversion: 1.0.0\nappVersion: \"0.26.0\"\n",
		"apps/app-spoolman/spoolman/values.yaml": "image:\n  repository: ghcr.io/donkie/spoolman\n  # -- pinned\n  tag: \"0.26.0\"\n",
		"apps/app-spoolman/spoolman/README.md":   "Runs spoolman 0.26.0.\n",
		"apps/app-media/sonarr/Chart.yaml":       "apiVersion: v2\nname: sonarr\nversion: 4.0.16\n# renovate: image=lscr.io/linuxserver/sonarr\nappVersion: \"4.0.17.2952-ls306\"\n",
		"apps/app-media/sonarr/values.yaml":      "image:\n  repository: lscr.io/linuxserver/sonarr\n  tag: \"\"\n",
		"apps/app-ddns/ddns/deployment.yaml":     "spec:\n  containers:\n    - name: ddns\n      image: docker.io/favonia/cloudflare-ddns:1.16.2\n",
		// Not picked; its pins must come out untouched.
		"apps/app-core/router/values.yaml": "image:\n  repository: traefik\n  tag: \"v3.4.0\"\n",
		"apps/app-core/router/Chart.yaml":  "apiVersion: v2\nname: router\nversion: 1.0.0\nappVersion: \"v3.4.0\"\n",
	}
	if helmErr == nil {
		files["charts-src/sub/Chart.yaml"] = "apiVersion: v2\nname: sub\nversion: 1.1.0\n"
		files["charts-src/sub/templates/cm.yaml"] = "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: sub\n"
		files["apps/app-kube/viewer/Chart.yaml"] = "apiVersion: v2\nname: viewer\nversion: 1.0.0\ndependencies:\n  - name: sub\n    version: 1.0.0\n    repository: file://../../../charts-src/sub\n"
	}
	write(t, seed, files)
	sh(t, seed, "git", "add", "-A")
	sh(t, seed, "git", "commit", "-q", "-m", "seed")
	sh(t, seed, "git", "remote", "add", "origin", origin)
	sh(t, seed, "git", "push", "-q", "origin", "main")

	cfg := &config.Config{
		Repo: "file://" + origin, Branch: "main", AppGlob: "apps/*/*", DataDir: filepath.Join(root, "data"),
		GitAuthorName: "FireBall1725", GitAuthorEmail: "727458+FireBall1725@users.noreply.github.com",
	}
	apps, err := inventory.Scan(seed, cfg.AppGlob)
	if err != nil {
		t.Fatal(err)
	}
	latest := map[string][2]string{
		"spoolman|image.tag":               {"0.26.1", "patch"},
		"sonarr|appVersion":                {"4.0.20.3014-ls325", "patch"},
		"ddns|deployment.yaml image":       {"1.17.1", "minor"},
		"viewer|dependencies[sub].version": {"1.1.0", "minor"},
		"router|image.tag":                 {"v3.7.13", "minor"},
		"router|appVersion":                {"", "current"},
	}
	for i := range apps {
		for j := range apps[i].Pins {
			p := &apps[i].Pins[j]
			if l, ok := latest[apps[i].Name+"|"+p.Field]; ok {
				p.Latest, p.Update = l[0], l[1]
			}
		}
	}

	conn, err := db.Open(cfg.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	repo := repository.New(conn)
	gh := &fakeGitHub{}
	srv := httptest.NewServer(gh)
	t.Cleanup(srv.Close)
	client := NewGitHub("tok", "o", "r")
	client.BaseURL = srv.URL
	scanner := &fakeScanner{result: scan.Result{State: models.ScanOK, Apps: apps}}
	svc := New(cfg, repo, scanner, client)
	svc.now = func() time.Time { return time.Date(2026, 9, 23, 2, 30, 0, 0, time.UTC) }
	return &fixture{origin: origin, svc: svc, scanner: scanner, gh: gh, repo: repo, helm: helmErr == nil}
}

func (f *fixture) gitOrigin(t *testing.T, args ...string) string {
	t.Helper()
	return sh(t, f.origin, append([]string{"git"}, args...)...)
}

func TestOpenPR(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	apps := []string{"apps/app-spoolman/spoolman", "apps/app-media/sonarr", "apps/app-ddns/ddns"}
	if f.helm {
		apps = append(apps, "apps/app-kube/viewer")
	}
	pr, err := f.svc.Create(ctx, Request{Title: "Bump a few things", AutoMerge: true, Apps: apps})
	if err != nil {
		t.Fatal(err)
	}
	if pr.Branch != "upstream/20260923-023000-"+map[bool]string{true: "4-apps", false: "3-apps"}[f.helm] {
		t.Errorf("branch %q", pr.Branch)
	}
	f.svc.process(ctx, pr.ID)

	prs, _ := f.repo.ListPRs(ctx, 1)
	got := prs[0]
	if got.State != models.PRMerged || got.Number != 7 || got.Error != "" {
		t.Fatalf("pr %+v", got)
	}
	if f.scanner.started != 1 {
		t.Error("a merged PR should trigger a rescan")
	}
	if f.gh.body["head"] != pr.Branch || f.gh.body["base"] != "main" || f.gh.body["title"] != "Bump a few things" {
		t.Errorf("PR request %v", f.gh.body)
	}
	if body, _ := f.gh.body["body"].(string); !strings.Contains(body, "| spoolman | `image.tag` | `0.26.0` | `0.26.1` |") {
		t.Errorf("PR body:\n%s", body)
	}

	changed := strings.Fields(f.gitOrigin(t, "diff", "--name-only", "main", pr.Branch))
	want := []string{
		"apps/app-ddns/ddns/deployment.yaml",
		"apps/app-media/sonarr/Chart.yaml",
		"apps/app-spoolman/spoolman/Chart.yaml",
		"apps/app-spoolman/spoolman/README.md",
		"apps/app-spoolman/spoolman/values.yaml",
	}
	if f.helm {
		want = append(want, "apps/app-kube/viewer/Chart.lock", "apps/app-kube/viewer/Chart.yaml", "apps/app-kube/viewer/charts/sub-1.1.0.tgz")
	}
	sort.Strings(want)
	if strings.Join(changed, "\n") != strings.Join(want, "\n") {
		t.Errorf("changed files:\n%s\nwant:\n%s", strings.Join(changed, "\n"), strings.Join(want, "\n"))
	}

	show := func(file string) string { return f.gitOrigin(t, "show", pr.Branch+":"+file) }
	if v := show("apps/app-spoolman/spoolman/values.yaml"); !strings.Contains(v, "  tag: \"0.26.1\"\n") || !strings.Contains(v, "# -- pinned") {
		t.Errorf("values.yaml:\n%s", v)
	}
	if c := show("apps/app-spoolman/spoolman/Chart.yaml"); !strings.Contains(c, "version: 1.0.1\n") || !strings.Contains(c, `appVersion: "0.26.1"`) {
		t.Errorf("spoolman Chart.yaml:\n%s", c)
	}
	if c := show("apps/app-media/sonarr/Chart.yaml"); !strings.Contains(c, `appVersion: "4.0.20.3014-ls325"`) || !strings.Contains(c, "version: 4.0.17\n") || !strings.Contains(c, "# renovate:") {
		t.Errorf("sonarr Chart.yaml:\n%s", c)
	}
	if r := show("apps/app-spoolman/spoolman/README.md"); r != "Runs spoolman 0.26.1.\n" {
		t.Errorf("README:\n%s", r)
	}
	if d := show("apps/app-ddns/ddns/deployment.yaml"); !strings.Contains(d, "cloudflare-ddns:1.17.1") {
		t.Errorf("deployment.yaml:\n%s", d)
	}

	msg := f.gitOrigin(t, "log", "-1", "--format=%an <%ae>%n%B", pr.Branch)
	for _, want := range []string{
		"FireBall1725 <727458+FireBall1725@users.noreply.github.com>",
		"Bump a few things",
		"spoolman image.tag: 0.26.0 -> 0.26.1",
		"Signed-off-by: FireBall1725 <727458+FireBall1725@users.noreply.github.com>",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("commit message missing %q:\n%s", want, msg)
		}
	}
	if strings.Contains(strings.ToLower(msg), "claude") || strings.Contains(msg, "Co-Authored-By") {
		t.Errorf("commit carries AI attribution:\n%s", msg)
	}

	// The work clone is removed afterwards.
	if entries, _ := os.ReadDir(filepath.Join(f.svc.cfg.DataDir, "work")); len(entries) != 0 {
		t.Errorf("work dir left behind: %v", entries)
	}
}

func TestRepoMovedSinceTheScan(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	// Someone bumps spoolman by hand after the scan.
	clone := filepath.Join(t.TempDir(), "c")
	sh(t, filepath.Dir(clone), "git", "clone", "-q", f.origin, clone)
	write(t, clone, map[string]string{"apps/app-spoolman/spoolman/values.yaml": "image:\n  repository: ghcr.io/donkie/spoolman\n  tag: \"0.26.1\"\n"})
	sh(t, clone, "git", "commit", "-qam", "by hand")
	sh(t, clone, "git", "push", "-q", "origin", "main")

	pr, err := f.svc.Create(ctx, Request{Apps: []string{"apps/app-spoolman/spoolman"}})
	if err != nil {
		t.Fatal(err)
	}
	if pr.Title != "spoolman: 0.26.0 -> 0.26.1" {
		t.Errorf("default title %q", pr.Title)
	}
	f.svc.process(ctx, pr.ID)

	prs, _ := f.repo.ListPRs(ctx, 1)
	if prs[0].State != models.PRFailed || !strings.Contains(prs[0].Error, "rescan") {
		t.Fatalf("pr %+v", prs[0])
	}
	if branches := f.gitOrigin(t, "branch", "--list", "upstream/*"); strings.TrimSpace(branches) != "" {
		t.Errorf("a failed PR pushed a branch: %s", branches)
	}
	if len(f.gh.calls) != 0 {
		t.Errorf("GitHub was called: %v", f.gh.calls)
	}
}

func TestCreateRejects(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if _, err := f.svc.Create(ctx, Request{Apps: []string{"apps/app-core/router-missing"}}); err != ErrNothingToBump {
		t.Errorf("unknown app: %v", err)
	}
	f.svc.gh = nil
	if _, err := f.svc.Create(ctx, Request{Apps: []string{"apps/app-spoolman/spoolman"}}); err != ErrNotConfigured {
		t.Errorf("no GitHub: %v", err)
	}
}

func TestRefreshMarksMergedPRs(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	id, _ := f.repo.CreatePR(ctx, models.PullRequest{Title: "x", Branch: "b", State: models.PRQueued, Items: []models.BumpItem{}})
	_ = f.repo.UpdatePR(ctx, models.PullRequest{ID: id, State: models.PROpen, Number: 7})
	f.svc.Refresh(ctx)
	prs, _ := f.repo.ListPRs(ctx, 1)
	if prs[0].State != models.PRMerged {
		t.Errorf("state %s", prs[0].State)
	}
}

func TestParseGitHubRepo(t *testing.T) {
	for in, want := range map[string]string{
		"https://github.com/FireBall1725/homelab-applications.git": "FireBall1725/homelab-applications",
		"https://github.com/o/r":                                   "o/r",
		"https://gitlab.com/o/r.git":                               "",
		"git@github.com:o/r.git":                                   "",
		"https://github.com/o/r/tree/main":                         "",
	} {
		o, n, ok := ParseGitHubRepo(in)
		got := ""
		if ok {
			got = o + "/" + n
		}
		if got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
}
