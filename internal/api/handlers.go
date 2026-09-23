// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

package api

import (
	"encoding/json"
	"net/http"

	"github.com/fireball1725/upstream/internal/config"
	"github.com/fireball1725/upstream/internal/version"
)

type handlers struct {
	cfg *config.Config
}

type statusResponse struct {
	Version  string   `json:"version"`
	Repo     string   `json:"repo"`
	Branch   string   `json:"branch"`
	AppGlob  string   `json:"appGlob"`
	Schedule string   `json:"schedule"`
	Problems []string `json:"problems"`
}

func (h *handlers) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": version.String()})
}

func (h *handlers) status(w http.ResponseWriter, _ *http.Request) {
	problems := []string{}
	for _, p := range h.cfg.Problems() {
		problems = append(problems, p.Error())
	}
	writeJSON(w, http.StatusOK, statusResponse{
		Version:  version.String(),
		Repo:     h.cfg.Repo,
		Branch:   h.cfg.Branch,
		AppGlob:  h.cfg.AppGlob,
		Schedule: h.cfg.Schedule,
		Problems: problems,
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
