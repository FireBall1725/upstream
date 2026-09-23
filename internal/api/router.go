// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

// Package api holds the JSON endpoints the embedded UI calls.
package api

import (
	"net/http"

	"github.com/fireball1725/upstream/internal/config"
	"github.com/fireball1725/upstream/internal/scan"
	"github.com/fireball1725/upstream/internal/ui"
)

// NewRouter wires every route; reading this file gives the whole HTTP surface.
func NewRouter(cfg *config.Config, scans *scan.Service) http.Handler {
	h := &handlers{cfg: cfg, scans: scans}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("GET /api/status", h.status)
	mux.HandleFunc("GET /api/scan", h.latestScan)
	mux.HandleFunc("POST /api/scan", h.startScan)
	mux.Handle("/api/", http.NotFoundHandler())

	mux.Handle("/", ui.Handler())
	return mux
}
