// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

// Package scan keeps a local clone of the GitOps repo current and runs the inventory over it.
package scan

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fireball1725/upstream/internal/config"
	"github.com/fireball1725/upstream/internal/inventory"
	"github.com/fireball1725/upstream/internal/sources"
	"github.com/fireball1725/upstream/internal/version"
)

// State is where the latest scan got to.
type State string

const (
	StateNever   State = "never"
	StateRunning State = "running"
	StateOK      State = "ok"
	StateFailed  State = "failed"
)

// Result is the latest scan. Apps holds the last successful scan even while a new one runs or after one fails.
type Result struct {
	State      State           `json:"state"`
	StartedAt  *time.Time      `json:"startedAt,omitempty"`
	FinishedAt *time.Time      `json:"finishedAt,omitempty"`
	Error      string          `json:"error,omitempty"`
	Commit     string          `json:"commit,omitempty"`
	CommitDate string          `json:"commitDate,omitempty"`
	Apps       []inventory.App `json:"apps"`
	History    []Summary       `json:"history"`
}

// Summary is one finished scan, kept for the Scans tab.
type Summary struct {
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
	State      State     `json:"state"`
	Error      string    `json:"error,omitempty"`
	Commit     string    `json:"commit,omitempty"`
	Apps       int       `json:"apps"`
	Checked    int       `json:"checked"`
	Updates    int       `json:"updates"`
	Errors     int       `json:"errors"`
}

// historySize is how many scans the Scans tab lists; it resets on restart until SQLite lands.
const historySize = 20

// ErrNotConfigured means there's no repo to scan.
var ErrNotConfigured = errors.New("UPSTREAM_REPO is not set, so there is nothing to scan")

// Service runs one scan at a time and remembers the result.
type Service struct {
	cfg     *config.Config
	timeout time.Duration
	// skipLookups keeps tests off the network.
	skipLookups bool

	mu     sync.Mutex
	result Result
}

func New(cfg *config.Config) *Service {
	return &Service{cfg: cfg, timeout: 5 * time.Minute, result: Result{State: StateNever, Apps: []inventory.App{}, History: []Summary{}}}
}

// Latest returns a copy of the latest result.
func (s *Service) Latest() Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.result
}

// Start begins a scan in the background. It returns false if one is already running.
func (s *Service) Start(ctx context.Context) (bool, error) {
	if s.cfg.Repo == "" {
		return false, ErrNotConfigured
	}
	s.mu.Lock()
	if s.result.State == StateRunning {
		s.mu.Unlock()
		return false, nil
	}
	now := time.Now()
	s.result.State, s.result.StartedAt, s.result.FinishedAt, s.result.Error = StateRunning, &now, nil, ""
	s.mu.Unlock()

	// The scan outlives the HTTP request that started it, so it gets its own deadline.
	go s.run(context.WithoutCancel(ctx))
	return true, nil
}

func (s *Service) run(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	commit, date, apps, err := s.scan(ctx)
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.result.FinishedAt = &now
	sum := Summary{StartedAt: *s.result.StartedAt, FinishedAt: now, Commit: commit}
	if err != nil {
		s.result.State, s.result.Error = StateFailed, err.Error()
		sum.State, sum.Error = StateFailed, err.Error()
		slog.Error("scan failed", "error", err)
	} else {
		s.result.State, s.result.Commit, s.result.CommitDate, s.result.Apps = StateOK, commit, date, apps
		sum.State, sum.Apps = StateOK, len(apps)
		for _, a := range apps {
			for _, p := range a.Pins {
				switch p.Update {
				case UpdateError:
					sum.Errors++
				case UpdateUnchecked:
				case UpdateCurrent:
					sum.Checked++
				default:
					sum.Checked++
					sum.Updates++
				}
			}
		}
		slog.Info("scan finished", "commit", commit, "apps", sum.Apps, "updates", sum.Updates, "errors", sum.Errors, "took", now.Sub(sum.StartedAt).Round(time.Millisecond).String())
	}
	s.result.History = append([]Summary{sum}, s.result.History...)
	if len(s.result.History) > historySize {
		s.result.History = s.result.History[:historySize]
	}
}

func (s *Service) scan(ctx context.Context) (commit, date string, apps []inventory.App, err error) {
	dir := filepath.Join(s.cfg.DataDir, "repo")
	if err := s.sync(ctx, dir); err != nil {
		return "", "", nil, err
	}
	out, err := s.git(ctx, dir, "log", "-1", "--format=%H %cI")
	if err != nil {
		return "", "", nil, err
	}
	commit, date, _ = strings.Cut(strings.TrimSpace(out), " ")
	apps, err = inventory.Scan(dir, s.cfg.AppGlob)
	if err != nil {
		return "", "", nil, err
	}
	if !s.skipLookups {
		checkApps(ctx, sources.New("upstream/"+version.String()), apps)
	}
	return commit, date, apps, nil
}

// sync makes dir a shallow clone of the configured branch, recloning if the repo URL changed.
func (s *Service) sync(ctx context.Context, dir string) error {
	if origin, err := s.git(ctx, dir, "remote", "get-url", "origin"); err != nil || strings.TrimSpace(origin) != s.cfg.Repo {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			return err
		}
		_, err := s.git(ctx, "", "clone", "--depth", "1", "--single-branch", "--branch", s.cfg.Branch, s.cfg.Repo, dir)
		return err
	}
	for _, args := range [][]string{
		{"fetch", "--depth", "1", "origin", s.cfg.Branch},
		{"reset", "--hard", "FETCH_HEAD"},
		{"clean", "-fdx"},
	} {
		if _, err := s.git(ctx, dir, args...); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		if _, err := os.Stat(dir); err != nil {
			return "", err
		}
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	cmd.Env = append(cmd.Env, s.authEnv()...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// authEnv passes the token as a header through git's env config, so it never lands in
// .git/config, the process list or an error message.
func (s *Service) authEnv() []string {
	u, err := url.Parse(s.cfg.Repo)
	if s.cfg.GitHubToken == "" || err != nil || u.Scheme != "https" || u.Host != "github.com" {
		return nil
	}
	basic := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + s.cfg.GitHubToken))
	return []string{
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=http.https://github.com/.extraheader",
		"GIT_CONFIG_VALUE_0=AUTHORIZATION: basic " + basic,
	}
}
