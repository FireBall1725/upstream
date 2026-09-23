// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

// Package api holds the JSON endpoints the embedded UI calls.
package api

import (
	"net/http"

	"github.com/fireball1725/upstream/internal/bump"
	"github.com/fireball1725/upstream/internal/config"
	"github.com/fireball1725/upstream/internal/repository"
	"github.com/fireball1725/upstream/internal/scan"
	"github.com/fireball1725/upstream/internal/ui"
)

// NewRouter wires every route; reading this file gives the whole HTTP surface.
// There's no login: the chart puts this behind an internal-only ingress.
func NewRouter(cfg *config.Config, repo *repository.Repo, scans *scan.Service, prs *bump.Service) http.Handler {
	h := &handlers{cfg: cfg, repo: repo, scans: scans, prs: prs}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("GET /metrics", h.metrics)
	mux.HandleFunc("GET /api/status", h.status)
	mux.HandleFunc("GET /api/scan", h.latestScan)
	mux.HandleFunc("POST /api/scan", h.startScan)
	mux.HandleFunc("GET /api/prs", h.listPRs)
	mux.HandleFunc("POST /api/prs", h.createPR)
	mux.HandleFunc("GET /api/skips", h.listSkips)
	mux.HandleFunc("POST /api/skips", h.addSkip)
	mux.HandleFunc("DELETE /api/skips", h.deleteSkip)
	mux.Handle("/api/", http.NotFoundHandler())

	mux.Handle("/", ui.Handler())
	return mux
}
