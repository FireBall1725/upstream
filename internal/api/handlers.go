// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/fireball1725/upstream/internal/config"
	"github.com/fireball1725/upstream/internal/scan"
	"github.com/fireball1725/upstream/internal/version"
	"github.com/robfig/cron/v3"
)

type handlers struct {
	cfg   *config.Config
	scans *scan.Service
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
	res := statusResponse{
		Version:   version.String(),
		Repo:      h.cfg.Repo,
		Branch:    h.cfg.Branch,
		AppGlob:   h.cfg.AppGlob,
		Schedule:  h.cfg.Schedule,
		TimeZone:  time.Local.String(),
		HasToken:  h.cfg.GitHubToken != "",
		GitAuthor: h.cfg.GitAuthorName,
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
		writeJSON(w, code, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, h.scans.Latest())
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
