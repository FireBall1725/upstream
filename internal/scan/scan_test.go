// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

package scan

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/fireball1725/upstream/internal/config"
)

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out)
	}
}

func commitApp(t *testing.T, repo, name, tag string) {
	t.Helper()
	dir := filepath.Join(repo, "apps", "app-test", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte("apiVersion: v2\nname: "+name+"\nversion: 1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "values.yaml"), []byte("image:\n  repository: example/"+name+"\n  tag: \""+tag+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, repo, "git", "add", "-A")
	run(t, repo, "git", "commit", "-q", "-m", name+" "+tag)
}

func waitDone(t *testing.T, s *Service) Result {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if r := s.Latest(); r.State != StateRunning {
			return r
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("scan did not finish")
	return Result{}
}

func TestService(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	repo := t.TempDir()
	run(t, repo, "git", "init", "-q", "-b", "main")
	commitApp(t, repo, "one", "1.0.0")

	cfg := &config.Config{Repo: "file://" + repo, Branch: "main", AppGlob: "apps/*/*", DataDir: t.TempDir()}
	s := New(cfg)

	if started, err := s.Start(context.Background()); !started || err != nil {
		t.Fatalf("start: %v %v", started, err)
	}
	if started, _ := s.Start(context.Background()); started {
		t.Error("a second scan started while the first was running")
	}
	r := waitDone(t, s)
	if r.State != StateOK || len(r.Apps) != 1 || r.Apps[0].Pins[0].Version != "1.0.0" || len(r.Commit) != 40 {
		t.Fatalf("first scan %+v", r)
	}

	// A new commit shows up on the next scan through fetch, not a fresh clone.
	commitApp(t, repo, "one", "1.1.0")
	commitApp(t, repo, "two", "2.0.0")
	if _, err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	r = waitDone(t, s)
	if r.State != StateOK || len(r.Apps) != 2 || r.Apps[0].Pins[0].Version != "1.1.0" {
		t.Fatalf("second scan %+v", r)
	}

	// A failed scan keeps the last good apps and says why.
	cfg.Branch = "no-such-branch"
	cfg.Repo = "file://" + filepath.Join(repo, "missing")
	if _, err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	r = waitDone(t, s)
	if r.State != StateFailed || r.Error == "" || len(r.Apps) != 2 {
		t.Fatalf("failed scan %+v", r)
	}
}

func TestAuthEnvOnlyForGitHubHTTPS(t *testing.T) {
	for repo, want := range map[string]bool{
		"https://github.com/o/r.git":     true,
		"http://github.com/o/r.git":      false,
		"https://gitlab.com/o/r.git":     false,
		"git@github.com:o/r.git":         false,
		"https://github.com.evil/o/r":    false,
		"https://example.com/github.com": false,
	} {
		s := New(&config.Config{Repo: repo, GitHubToken: "secret"})
		if got := len(s.authEnv()) > 0; got != want {
			t.Errorf("%s: sends token %v, want %v", repo, got, want)
		}
	}
}
