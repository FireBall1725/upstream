// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

// Command upstream serves the update tracker's API and web UI from one binary.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	// Embeds the zone database so TZ works in a minimal image with no /usr/share/zoneinfo.
	_ "time/tzdata"

	"github.com/fireball1725/upstream/internal/api"
	"github.com/fireball1725/upstream/internal/bump"
	"github.com/fireball1725/upstream/internal/config"
	"github.com/fireball1725/upstream/internal/db"
	"github.com/fireball1725/upstream/internal/repository"
	"github.com/fireball1725/upstream/internal/scan"
	"github.com/fireball1725/upstream/internal/version"
	"github.com/robfig/cron/v3"
)

func main() {
	cfg := config.Load()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel})))

	slog.Info("upstream starting", "version", version.String(), "repo", cfg.Repo, "branch", cfg.Branch, "glob", cfg.AppGlob)
	for _, p := range cfg.Problems() {
		slog.Warn("config", "problem", p.Error())
	}

	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	conn, err := db.Open(cfg.DataDir)
	if err != nil {
		slog.Error("database", "error", err)
		os.Exit(1)
	}
	defer func() { _ = conn.Close() }()
	repo := repository.New(conn)

	scans := scan.New(cfg, repo)
	if err := scans.Load(ctx); err != nil {
		slog.Warn("couldn't load the last scan", "error", err)
	}

	var gh *bump.GitHub
	if owner, name, ok := bump.ParseGitHubRepo(cfg.Repo); ok && cfg.GitHubToken != "" {
		gh = bump.NewGitHub(cfg.GitHubToken, owner, name)
	}
	prs := bump.New(cfg, repo, scans, gh)
	go prs.Run(ctx)
	// Each scan also checks whether open PRs were merged or closed.
	scans.OnFinish = func(ctx context.Context, _ scan.Result) { prs.Refresh(ctx) }

	// Scan at boot so the page is current; the stored result shows until it finishes.
	if _, err := scans.Start(ctx); err != nil {
		slog.Warn("no scan at startup", "reason", err.Error())
	}

	// cron uses the process time zone, so TZ decides when "0 */6 * * *" fires.
	sched := cron.New()
	if cfg.Repo != "" {
		if _, err := sched.AddFunc(cfg.Schedule, func() {
			if _, err := scans.Start(ctx); err != nil {
				slog.Error("scheduled scan", "error", err)
			}
		}); err != nil {
			slog.Error("scan schedule not started", "schedule", cfg.Schedule, "error", err)
		}
	}
	sched.Start()
	defer sched.Stop()

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           api.NewRouter(cfg, repo, scans, prs),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("listening", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("forced shutdown", "error", err)
	}
}
