// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/fireball1725/upstream/internal/bump"
	"github.com/fireball1725/upstream/internal/config"
	"github.com/fireball1725/upstream/internal/models"
	"github.com/fireball1725/upstream/internal/repository"
	"github.com/fireball1725/upstream/internal/scan"
	"github.com/fireball1725/upstream/internal/version"
	"github.com/robfig/cron/v3"
)

type handlers struct {
	cfg   *config.Config
	repo  *repository.Repo
	scans *scan.Service
	prs   *bump.Service
}

type errorResponse struct {
	Error string `json:"error"`
}

type statusResponse struct {
	Version   string     `json:"version"`
	Repo      string     `json:"repo"`
	Branch    string     `json:"branch"`
	AppGlob   string     `json:"appGlob"`
	Schedule  string     `json:"schedule"`
	NextScan  *time.Time `json:"nextScan,omitempty"`
	TimeZone  string     `json:"timeZone"`
	HasToken  bool       `json:"hasToken"`
	GitAuthor string     `json:"gitAuthor,omitempty"`
	PRsReady  bool       `json:"prsReady"`
	Problems  []string   `json:"problems"`
}

func (h *handlers) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": version.String()})
}

func (h *handlers) status(w http.ResponseWriter, _ *http.Request) {
	problems := []string{}
	for _, p := range h.cfg.Problems() {
		problems = append(problems, p.Error())
	}
	if _, _, ok := bump.ParseGitHubRepo(h.cfg.Repo); h.cfg.Repo != "" && !ok {
		problems = append(problems, "Bump PRs only work with an https://github.com repo; scanning still works.")
	}
	if h.scans.TokenRejected() {
		problems = append(problems, "GitHub rejected GITHUB_TOKEN; it may have expired. Scans still run without it, but bump PRs won't work until it's replaced.")
	}
	res := statusResponse{
		Version:   version.String(),
		Repo:      h.cfg.Repo,
		Branch:    h.cfg.Branch,
		AppGlob:   h.cfg.AppGlob,
		Schedule:  h.cfg.Schedule,
		TimeZone:  time.Local.String(),
		HasToken:  h.cfg.GitHubToken != "",
		GitAuthor: h.cfg.GitAuthorName,
		PRsReady:  h.prs.Ready() == nil && !h.scans.TokenRejected(),
		Problems:  problems,
	}
	if sched, err := cron.ParseStandard(h.cfg.Schedule); err == nil && h.cfg.Repo != "" {
		next := sched.Next(time.Now())
		res.NextScan = &next
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *handlers) latestScan(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.scans.Latest())
}

// startScan answers 202 whether it started a scan or one was already running; the client polls GET either way.
func (h *handlers) startScan(w http.ResponseWriter, r *http.Request) {
	if _, err := h.scans.Start(r.Context()); err != nil {
		code := http.StatusInternalServerError
		if errors.Is(err, scan.ErrNotConfigured) {
			code = http.StatusConflict
		}
		writeError(w, code, err)
		return
	}
	writeJSON(w, http.StatusAccepted, h.scans.Latest())
}

func (h *handlers) listPRs(w http.ResponseWriter, r *http.Request) {
	prs, err := h.repo.ListPRs(r.Context(), 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, prs)
}

func (h *handlers) createPR(w http.ResponseWriter, r *http.Request) {
	var req bump.Request
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("the request body isn't valid JSON"))
		return
	}
	pr, err := h.prs.Create(r.Context(), req)
	switch {
	case errors.Is(err, bump.ErrNotConfigured), errors.Is(err, bump.ErrNothingToBump):
		writeError(w, http.StatusConflict, err)
	case err != nil:
		writeError(w, http.StatusInternalServerError, err)
	default:
		writeJSON(w, http.StatusAccepted, pr)
	}
}

func (h *handlers) listSkips(w http.ResponseWriter, r *http.Request) {
	skips, err := h.repo.ListSkips(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, skips)
}

func (h *handlers) decodeSkip(w http.ResponseWriter, r *http.Request) (models.Skip, bool) {
	var s models.Skip
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&s); err != nil || s.AppDir == "" || s.Field == "" || s.Version == "" {
		writeError(w, http.StatusBadRequest, errors.New("a skip needs appDir, field and version"))
		return s, false
	}
	return s, true
}

// addSkip hides a version and rescans, so the app shows its next newest version or current.
func (h *handlers) addSkip(w http.ResponseWriter, r *http.Request) {
	s, ok := h.decodeSkip(w, r)
	if !ok {
		return
	}
	if err := h.repo.AddSkip(r.Context(), s); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	_, _ = h.scans.Start(r.Context())
	writeJSON(w, http.StatusCreated, s)
}

func (h *handlers) deleteSkip(w http.ResponseWriter, r *http.Request) {
	s, ok := h.decodeSkip(w, r)
	if !ok {
		return
	}
	if err := h.repo.DeleteSkip(r.Context(), s); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	_, _ = h.scans.Start(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

func writeError(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, errorResponse{Error: err.Error()})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
