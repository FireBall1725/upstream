// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

package bump

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/fireball1725/upstream/internal/config"
	"github.com/fireball1725/upstream/internal/inventory"
	"github.com/fireball1725/upstream/internal/models"
	"github.com/fireball1725/upstream/internal/repository"
	"github.com/fireball1725/upstream/internal/scan"
)

// Scanner is the part of the scan service a bump needs: the latest result and a way to rescan.
type Scanner interface {
	Latest() scan.Result
	Start(context.Context) (bool, error)
}

// Request is what the UI sends: the apps to bump and how.
type Request struct {
	Title     string   `json:"title"`
	AutoMerge bool     `json:"autoMerge"`
	Apps      []string `json:"apps"`
}

var (
	ErrNotConfigured = errors.New("bump PRs need a GitHub repo, GITHUB_TOKEN and a commit author")
	ErrNothingToBump = errors.New("none of those apps has an update in the latest scan")
)

// Service opens bump PRs one at a time, in the background.
type Service struct {
	cfg   *config.Config
	repo  *repository.Repo
	scans Scanner
	gh    *GitHub
	queue chan int64
	now   func() time.Time
}

// New returns a Service. gh is nil when the repo isn't on GitHub or there's no token.
func New(cfg *config.Config, repo *repository.Repo, scans Scanner, gh *GitHub) *Service {
	return &Service{cfg: cfg, repo: repo, scans: scans, gh: gh, queue: make(chan int64, 64), now: time.Now}
}

// Ready reports whether PRs can be opened, and why not.
func (s *Service) Ready() error {
	if s.gh == nil || s.cfg.GitAuthorName == "" || s.cfg.GitAuthorEmail == "" {
		return ErrNotConfigured
	}
	return nil
}

// Run works the queue until ctx ends. Anything left queued or running from before a restart fails
// first, and work clones a crash left behind are removed.
func (s *Service) Run(ctx context.Context) {
	_ = os.RemoveAll(filepath.Join(s.cfg.DataDir, "work"))
	if prs, err := s.repo.ListPRs(ctx, 200); err == nil {
		for _, p := range prs {
			if p.State == models.PRQueued || p.State == models.PRRunning {
				p.State, p.Error = models.PRFailed, "Upstream restarted before this PR finished. Nothing was pushed unless it shows a PR number."
				_ = s.repo.UpdatePR(ctx, p)
			}
		}
	}
	for {
		select {
		case <-ctx.Done():
			return
		case id := <-s.queue:
			s.process(ctx, id)
		}
	}
}

// Create checks the request against the latest scan, records the PR and queues it.
func (s *Service) Create(ctx context.Context, req Request) (models.PullRequest, error) {
	if err := s.Ready(); err != nil {
		return models.PullRequest{}, err
	}
	items := s.itemsFor(req.Apps)
	if len(items) == 0 {
		return models.PullRequest{}, ErrNothingToBump
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = defaultTitle(items)
	}
	now := s.now()
	pr := models.PullRequest{
		CreatedAt: now,
		UpdatedAt: now,
		Title:     title,
		Branch:    s.branchName(items),
		State:     models.PRQueued,
		AutoMerge: req.AutoMerge,
		Items:     items,
	}
	id, err := s.repo.CreatePR(ctx, pr)
	if err != nil {
		return pr, err
	}
	pr.ID = id
	select {
	case s.queue <- id:
	default:
		pr.State, pr.Error = models.PRFailed, "Too many PRs waiting; try again in a minute."
		_ = s.repo.UpdatePR(ctx, pr)
	}
	return pr, nil
}

var updatable = map[string]bool{"major": true, "minor": true, "patch": true, "rebuild": true}

func (s *Service) itemsFor(dirs []string) []models.BumpItem {
	want := map[string]bool{}
	for _, d := range dirs {
		want[d] = true
	}
	var items []models.BumpItem
	for _, a := range s.scans.Latest().Apps {
		if !want[a.Dir] {
			continue
		}
		for _, p := range a.Pins {
			if !updatable[p.Update] || p.Latest == "" || p.Latest == p.Version {
				continue
			}
			src := p.Image
			if p.Kind == inventory.KindChart {
				src = p.ChartRepo + " " + p.Name
			}
			items = append(items, models.BumpItem{
				AppDir: a.Dir, App: a.Name, Field: p.Field, Kind: string(p.Kind), Source: src,
				From: p.Version, To: p.Latest, Bump: p.Update,
			})
		}
	}
	return items
}

func defaultTitle(items []models.BumpItem) string {
	apps := appNames(items)
	if len(items) == 1 {
		return fmt.Sprintf("%s: %s -> %s", items[0].App, items[0].From, items[0].To)
	}
	if len(apps) == 1 {
		return fmt.Sprintf("%s: bump %d versions", apps[0], len(items))
	}
	return fmt.Sprintf("Bump %s", strings.Join(apps, ", "))
}

func appNames(items []models.BumpItem) []string {
	seen := map[string]bool{}
	var out []string
	for _, it := range items {
		if !seen[it.App] {
			seen[it.App] = true
			out = append(out, it.App)
		}
	}
	sort.Strings(out)
	return out
}

var branchUnsafe = regexp.MustCompile(`[^a-z0-9.-]+`)

func (s *Service) branchName(items []models.BumpItem) string {
	apps := appNames(items)
	slug := apps[0]
	if len(apps) > 1 {
		slug = fmt.Sprintf("%d-apps", len(apps))
	}
	return fmt.Sprintf("upstream/%s-%s", s.now().Format("20060102-150405"), strings.Trim(branchUnsafe.ReplaceAllString(strings.ToLower(slug), "-"), "-"))
}

// Refresh asks GitHub about open PRs so merged and closed ones stop showing as open.
func (s *Service) Refresh(ctx context.Context) {
	if s.gh == nil {
		return
	}
	prs, err := s.repo.ListPRs(ctx, 200)
	if err != nil {
		return
	}
	for _, p := range prs {
		if p.State != models.PROpen || p.Number == 0 {
			continue
		}
		got, err := s.gh.GetPR(ctx, p.Number)
		if err != nil {
			slog.Warn("refresh PR", "number", p.Number, "error", err)
			continue
		}
		switch {
		case got.Merged:
			p.State = models.PRMerged
		case got.State == "closed":
			p.State = models.PRClosed
		default:
			continue
		}
		_ = s.repo.UpdatePR(ctx, p)
	}
}

func (s *Service) process(ctx context.Context, id int64) {
	prs, err := s.repo.ListPRs(ctx, 200)
	if err != nil {
		slog.Error("load PR", "id", id, "error", err)
		return
	}
	var pr models.PullRequest
	for _, p := range prs {
		if p.ID == id {
			pr = p
		}
	}
	if pr.ID == 0 {
		return
	}
	pr.State = models.PRRunning
	_ = s.repo.UpdatePR(ctx, pr)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if err := s.open(ctx, &pr); err != nil {
		pr.State, pr.Error = models.PRFailed, err.Error()
		slog.Error("bump PR failed", "id", id, "title", pr.Title, "error", err)
	}
	_ = s.repo.UpdatePR(ctx, pr)
	if pr.State == models.PRMerged {
		if _, err := s.scans.Start(ctx); err != nil {
			slog.Warn("rescan after merge", "error", err)
		}
	}
}

// open does the work: clone, edit, verify, push, open the PR, and merge it if asked.
func (s *Service) open(ctx context.Context, pr *models.PullRequest) error {
	dir := filepath.Join(s.cfg.DataDir, "work", fmt.Sprintf("pr-%d", pr.ID))
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	auth := scan.AuthEnv(s.cfg)
	if _, err := scan.RunGit(ctx, "", auth, "clone", "--depth", "1", "--single-branch", "--branch", s.cfg.Branch, s.cfg.Repo, dir); err != nil {
		return err
	}
	if err := s.edit(ctx, dir, pr.Items); err != nil {
		return err
	}

	gitEnv := append(auth,
		"GIT_AUTHOR_NAME="+s.cfg.GitAuthorName, "GIT_AUTHOR_EMAIL="+s.cfg.GitAuthorEmail,
		"GIT_COMMITTER_NAME="+s.cfg.GitAuthorName, "GIT_COMMITTER_EMAIL="+s.cfg.GitAuthorEmail)
	if _, err := scan.RunGit(ctx, dir, gitEnv, "checkout", "-q", "-b", pr.Branch); err != nil {
		return err
	}
	if _, err := scan.RunGit(ctx, dir, gitEnv, "add", "-A"); err != nil {
		return err
	}
	if _, err := scan.RunGit(ctx, dir, gitEnv, "commit", "-q", "-s", "-m", pr.Title, "-m", commitBody(pr.Items)); err != nil {
		return err
	}
	if _, err := scan.RunGit(ctx, dir, gitEnv, "push", "-q", "origin", "HEAD:refs/heads/"+pr.Branch); err != nil {
		return err
	}

	opened, err := s.gh.CreatePR(ctx, pr.Title, prBody(pr.Items), pr.Branch, s.cfg.Branch)
	if err != nil {
		return err
	}
	pr.Number, pr.URL, pr.State = opened.Number, opened.HTMLURL, models.PROpen
	_ = s.repo.UpdatePR(ctx, *pr)
	slog.Info("bump PR opened", "number", pr.Number, "title", pr.Title)

	if pr.AutoMerge {
		if err := s.gh.MergePR(ctx, pr.Number); err != nil {
			// The PR exists and is fine; say why it's still open rather than calling the whole thing failed.
			pr.Error = "Opened, but auto-merge failed: " + err.Error()
			return nil
		}
		pr.State = models.PRMerged
		s.gh.DeleteBranch(ctx, pr.Branch)
	}
	return nil
}

// edit applies every item in the clone, then proves the result before anything is committed.
func (s *Service) edit(ctx context.Context, dir string, items []models.BumpItem) error {
	before, err := inventory.Scan(dir, s.cfg.AppGlob)
	if err != nil {
		return err
	}
	pins := map[string]inventory.Pin{}
	appDirs := map[string]bool{}
	for _, a := range before {
		for _, p := range a.Pins {
			pins[a.Dir+"|"+p.Field] = p
		}
	}

	helmDirs := map[string]bool{}
	for _, it := range items {
		p, ok := pins[it.AppDir+"|"+it.Field]
		if !ok {
			return fmt.Errorf("%s: %s is gone from the repo since the scan", it.App, it.Field)
		}
		if p.Version != it.From {
			return fmt.Errorf("%s: %s is now %s, not %s; rescan and try again", it.App, it.Field, p.Version, it.From)
		}
		if err := applyItem(dir, it, p); err != nil {
			return fmt.Errorf("%s: %w", it.App, err)
		}
		appDirs[it.AppDir] = true
		if it.Kind == string(inventory.KindChart) {
			helmDirs[it.AppDir] = true
		}
	}

	for d := range appDirs {
		chart := filepath.Join(dir, filepath.FromSlash(d), "Chart.yaml")
		raw, err := os.ReadFile(chart)
		if err != nil {
			continue
		}
		next, from, to := BumpChartVersion(string(raw))
		if err := os.WriteFile(chart, []byte(next), 0o644); err != nil {
			return err
		}
		if from == "" {
			continue
		}
		readme := filepath.Join(dir, filepath.FromSlash(d), "README.md")
		if raw, err := os.ReadFile(readme); err == nil {
			if updated := ReplaceChartVersionRow(string(raw), from, to); updated != string(raw) {
				if err := os.WriteFile(readme, []byte(updated), 0o644); err != nil {
					return err
				}
			}
		}
	}
	for d := range helmDirs {
		if _, err := s.helm(ctx, filepath.Join(dir, filepath.FromSlash(d)), "dependency", "update", "."); err != nil {
			return err
		}
	}

	return s.verify(ctx, dir, items, before, appDirs)
}

func applyItem(dir string, it models.BumpItem, p inventory.Pin) error {
	appDir := filepath.Join(dir, filepath.FromSlash(it.AppDir))
	file := filepath.Join(dir, filepath.FromSlash(p.File))
	raw, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	content := string(raw)

	switch {
	case p.Kind == inventory.KindChart:
		// A stale vendored chart's line holds the declared version, not the one that runs.
		lineHolds := p.Version
		if p.Declared != "" {
			lineHolds = p.Declared
		}
		content, err = SetDependencyVersion(content, p.Line, lineHolds, it.To)
	case p.Field == "appVersion":
		content, err = AppVersionIs(content, p.Line, p.Version, it.To)
	case strings.HasSuffix(p.Field, "image.tag"):
		content, err = SetImageTag(content, p.Line, p.Version, it.To)
		if err == nil && p.Field == "image.tag" {
			err = keepAppVersion(filepath.Join(appDir, "Chart.yaml"), it.To)
		}
	case strings.HasSuffix(p.Field, " image") || strings.Contains(p.Field, " image["):
		content, err = SetManifestImage(content, p.Line, p.Image, p.Version, it.To)
	default:
		err = fmt.Errorf("don't know how to edit %s", p.Field)
	}
	if err != nil {
		return err
	}
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		return err
	}

	readme := filepath.Join(appDir, "README.md")
	if raw, err := os.ReadFile(readme); err == nil {
		next := ReplaceVersionMentions(string(raw), it.From, it.To, p.Image)
		// The App Version row tracks appVersion, which only the top-level image moves.
		if p.Field == "appVersion" || p.Field == "image.tag" {
			next = SetAppVersionRow(next, it.To)
		}
		if next != string(raw) {
			return os.WriteFile(readme, []byte(next), 0o644)
		}
	}
	return nil
}

// keepAppVersion sets appVersion to the new tag, which also fixes any drift the scan reported.
func keepAppVersion(chart, to string) error {
	raw, err := os.ReadFile(chart)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	next, err := SetAppVersion(string(raw), to)
	if err != nil {
		return nil // no appVersion line to keep in step
	}
	return os.WriteFile(chart, []byte(next), 0o644)
}

// verify re-reads the clone: every item reads its new version, nothing else moved, only the
// apps' own files changed, and every touched chart still renders.
func (s *Service) verify(ctx context.Context, dir string, items []models.BumpItem, before []inventory.App, appDirs map[string]bool) error {
	after, err := inventory.Scan(dir, s.cfg.AppGlob)
	if err != nil {
		return err
	}
	now := map[string]inventory.Pin{}
	for _, a := range after {
		for _, p := range a.Pins {
			now[a.Dir+"|"+p.Field] = p
		}
	}
	targets := map[string]string{}
	for _, it := range items {
		targets[it.AppDir+"|"+it.Field] = it.To
	}
	for _, a := range before {
		for _, p := range a.Pins {
			key := a.Dir + "|" + p.Field
			got, ok := now[key]
			if !ok {
				return fmt.Errorf("check failed: %s %s disappeared after the edit", a.Name, p.Field)
			}
			if to, target := targets[key]; target {
				if got.Version != to || got.Declared != "" {
					return fmt.Errorf("check failed: %s %s reads %s after the edit, want %s", a.Name, p.Field, got.Version, to)
				}
			} else if got.Version != p.Version {
				return fmt.Errorf("check failed: %s %s changed from %s to %s, but it wasn't picked", a.Name, p.Field, p.Version, got.Version)
			}
		}
	}

	out, err := scan.RunGit(ctx, dir, nil, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return err
	}
	for line := range strings.SplitSeq(strings.TrimRight(out, "\n"), "\n") {
		if len(line) < 4 {
			continue
		}
		changed := strings.Trim(line[3:], `"`)
		if i := strings.Index(changed, " -> "); i >= 0 {
			changed = changed[i+4:]
		}
		inside := false
		for d := range appDirs {
			if strings.HasPrefix(changed, d+"/") {
				inside = true
			}
		}
		if !inside {
			return fmt.Errorf("check failed: %s changed, but it's outside the apps being bumped", changed)
		}
	}

	for d := range appDirs {
		appDir := filepath.Join(dir, filepath.FromSlash(d))
		if _, err := os.Stat(filepath.Join(appDir, "Chart.yaml")); err != nil {
			continue
		}
		if _, err := s.helm(ctx, appDir, "template", path.Base(d), "."); err != nil {
			return fmt.Errorf("check failed: %s no longer renders: %w", path.Base(d), err)
		}
	}
	return nil
}

func (s *Service) helm(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "helm", args...)
	cmd.Dir = dir
	home := filepath.Join(s.cfg.DataDir, "helm")
	cmd.Env = append(os.Environ(),
		"HELM_CACHE_HOME="+filepath.Join(home, "cache"),
		"HELM_CONFIG_HOME="+filepath.Join(home, "config"),
		"HELM_DATA_HOME="+filepath.Join(home, "data"))
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("helm %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
