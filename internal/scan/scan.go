// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

// Package scan keeps a local clone of the GitOps repo current and runs the inventory over it.
package scan

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
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
	"github.com/fireball1725/upstream/internal/models"
	"github.com/fireball1725/upstream/internal/repository"
	"github.com/fireball1725/upstream/internal/sources"
	"github.com/fireball1725/upstream/internal/version"
)

// Result is the latest scan. Apps holds the last successful scan even while a new one runs or after one fails.
type Result struct {
	State      models.ScanState     `json:"state"`
	StartedAt  *time.Time           `json:"startedAt,omitempty"`
	FinishedAt *time.Time           `json:"finishedAt,omitempty"`
	Error      string               `json:"error,omitempty"`
	Commit     string               `json:"commit,omitempty"`
	CommitDate string               `json:"commitDate,omitempty"`
	Apps       []inventory.App      `json:"apps"`
	History    []models.ScanSummary `json:"history"`
}

// historySize is how many scans the Scans tab lists.
const historySize = 20

// ErrNotConfigured means there's no repo to scan.
var ErrNotConfigured = errors.New("UPSTREAM_REPO is not set, so there is nothing to scan")

// Service runs one scan at a time, remembers the result and stores it.
type Service struct {
	cfg     *config.Config
	repo    *repository.Repo
	timeout time.Duration
	// skipLookups keeps tests off the network.
	skipLookups bool
	// OnFinish runs after every successful scan, outside the lock.
	OnFinish func(context.Context, Result)

	mu            sync.Mutex
	result        Result
	tokenRejected bool
	// gitMu serialises git commands in the shared clone.
	gitMu sync.Mutex
}

func New(cfg *config.Config, repo *repository.Repo) *Service {
	return &Service{
		cfg:     cfg,
		repo:    repo,
		timeout: 5 * time.Minute,
		result:  Result{State: models.ScanNever, Apps: []inventory.App{}, History: []models.ScanSummary{}},
	}
}

// Load restores the last stored result so the page isn't empty after a restart.
func (s *Service) Load(ctx context.Context) error {
	history, err := s.repo.RecentScans(ctx, historySize)
	if err != nil {
		return err
	}
	raw, err := s.repo.LatestResult(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.result.History = history
	if raw != nil {
		var r Result
		if err := json.Unmarshal(raw, &r); err != nil {
			return fmt.Errorf("stored scan: %w", err)
		}
		s.result.State, s.result.StartedAt, s.result.FinishedAt = models.ScanOK, r.StartedAt, r.FinishedAt
		s.result.Commit, s.result.CommitDate, s.result.Apps = r.Commit, r.CommitDate, r.Apps
	}
	return nil
}

// Latest returns a copy of the latest result.
func (s *Service) Latest() Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.result
}

// TokenRejected reports whether GitHub refused GITHUB_TOKEN on the last clone or fetch.
func (s *Service) TokenRejected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tokenRejected
}

// Start begins a scan in the background. It returns false if one is already running.
func (s *Service) Start(ctx context.Context) (bool, error) {
	if s.cfg.Repo == "" {
		return false, ErrNotConfigured
	}
	s.mu.Lock()
	if s.result.State == models.ScanRunning {
		s.mu.Unlock()
		return false, nil
	}
	now := time.Now()
	s.result.State, s.result.StartedAt, s.result.FinishedAt, s.result.Error = models.ScanRunning, &now, nil, ""
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
	s.result.FinishedAt = &now
	sum := models.ScanSummary{StartedAt: *s.result.StartedAt, FinishedAt: now, Commit: commit}
	var stored []byte
	if err != nil {
		s.result.State, s.result.Error = models.ScanFailed, err.Error()
		sum.State, sum.Error = models.ScanFailed, err.Error()
		slog.Error("scan failed", "error", err)
	} else {
		s.result.State, s.result.Commit, s.result.CommitDate, s.result.Apps = models.ScanOK, commit, date, apps
		sum.State, sum.Apps = models.ScanOK, len(apps)
		countPins(&sum, apps)
		keep := s.result
		keep.History = nil
		stored, _ = json.Marshal(keep)
		slog.Info("scan finished", "commit", commit, "apps", sum.Apps, "updates", sum.Updates, "errors", sum.Errors, "took", now.Sub(sum.StartedAt).Round(time.Millisecond).String())
	}
	s.result.History = append([]models.ScanSummary{sum}, s.result.History...)
	if len(s.result.History) > historySize {
		s.result.History = s.result.History[:historySize]
	}
	finished := s.result
	s.mu.Unlock()

	if err := s.repo.SaveScan(ctx, sum, stored); err != nil {
		slog.Error("save scan", "error", err)
	}
	if err == nil && s.OnFinish != nil {
		s.OnFinish(ctx, finished)
	}
}

func countPins(sum *models.ScanSummary, apps []inventory.App) {
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
}

func (s *Service) scan(ctx context.Context) (commit, date string, apps []inventory.App, err error) {
	dir := s.RepoDir()
	s.gitMu.Lock()
	err = s.sync(ctx, dir)
	if err == nil {
		var out string
		out, err = s.Git(ctx, dir, "log", "-1", "--format=%H %cI")
		commit, date, _ = strings.Cut(strings.TrimSpace(out), " ")
	}
	if err == nil {
		apps, err = inventory.Scan(dir, s.cfg.AppGlob)
	}
	s.gitMu.Unlock()
	if err != nil {
		return "", "", nil, err
	}

	skips, err := s.repo.ListSkips(ctx)
	if err != nil {
		return "", "", nil, err
	}
	if !s.skipLookups {
		checkApps(ctx, sources.New("upstream/"+version.String()), apps, skipIndex(skips))
	}
	return commit, date, apps, nil
}

// RepoDir is the shared clone of the GitOps repo.
func (s *Service) RepoDir() string { return filepath.Join(s.cfg.DataDir, "repo") }

// sync makes dir a shallow clone of the configured branch, recloning if the repo URL changed.
func (s *Service) sync(ctx context.Context, dir string) error {
	err := s.syncOnce(ctx, dir, true)
	if err != nil && s.cfg.GitHubToken != "" && authFailure(err) {
		// An expired token shouldn't stop scans of a public repo; say what happened and carry on.
		slog.Warn("GitHub rejected GITHUB_TOKEN; retrying without it", "error", err)
		s.mu.Lock()
		s.tokenRejected = true
		s.mu.Unlock()
		return s.syncOnce(ctx, dir, false)
	}
	if err == nil {
		s.mu.Lock()
		s.tokenRejected = false
		s.mu.Unlock()
	}
	return err
}

func authFailure(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "Authentication failed") || strings.Contains(msg, "Invalid username or token") || strings.Contains(msg, "403")
}

func (s *Service) syncOnce(ctx context.Context, dir string, auth bool) error {
	git := s.Git
	if !auth {
		git = s.gitAnon
	}
	if origin, err := s.Git(ctx, dir, "remote", "get-url", "origin"); err != nil || strings.TrimSpace(origin) != s.cfg.Repo {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			return err
		}
		_, err := git(ctx, "", "clone", "--depth", "1", "--single-branch", "--branch", s.cfg.Branch, s.cfg.Repo, dir)
		return err
	}
	for _, args := range [][]string{
		{"fetch", "--depth", "1", "origin", s.cfg.Branch},
		{"reset", "--hard", "FETCH_HEAD"},
		{"clean", "-fdx"},
	} {
		if _, err := git(ctx, dir, args...); err != nil {
			return err
		}
	}
	return nil
}

// Git runs git in dir with the token header, when there is one for this repo.
func (s *Service) Git(ctx context.Context, dir string, args ...string) (string, error) {
	return RunGit(ctx, dir, AuthEnv(s.cfg), args...)
}

func (s *Service) gitAnon(ctx context.Context, dir string, args ...string) (string, error) {
	return RunGit(ctx, dir, nil, args...)
}

// RunGit runs git in dir with extra environment and returns stdout, or stderr in the error.
func RunGit(ctx context.Context, dir string, env []string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		if _, err := os.Stat(dir); err != nil {
			return "", err
		}
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	cmd.Env = append(cmd.Env, env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// AuthEnv passes the token as a header through git's env config, so it never lands in
// .git/config, the process list or an error message.
func AuthEnv(cfg *config.Config) []string {
	u, err := url.Parse(cfg.Repo)
	if cfg.GitHubToken == "" || err != nil || u.Scheme != "https" || u.Host != "github.com" {
		return nil
	}
	basic := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + cfg.GitHubToken))
	return []string{
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=http.https://github.com/.extraheader",
		"GIT_CONFIG_VALUE_0=AUTHORIZATION: basic " + basic,
	}
}
