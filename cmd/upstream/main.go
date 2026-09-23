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
	"github.com/fireball1725/upstream/internal/config"
	"github.com/fireball1725/upstream/internal/version"
)

func main() {
	cfg := config.Load()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel})))

	slog.Info("upstream starting", "version", version.String(), "repo", cfg.Repo, "branch", cfg.Branch, "glob", cfg.AppGlob)
	for _, p := range cfg.Problems() {
		slog.Warn("config", "problem", p.Error())
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           api.NewRouter(cfg),
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
