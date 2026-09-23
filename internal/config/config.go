// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

// Package config reads runtime settings from the environment.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/robfig/cron/v3"
)

// Config holds everything Upstream needs to know about the repo it watches.
// Nothing here defaults to a particular homelab, so another GitOps repo only needs different env vars.
type Config struct {
	Addr     string
	LogLevel slog.Level
	DataDir  string

	// Repo is the git URL of the GitOps repo, cloned read-only for scans.
	Repo    string
	Branch  string
	AppGlob string

	// Schedule is a five-field cron expression in the process time zone (TZ).
	Schedule string

	// GitHubToken reads release notes and opens bump PRs.
	GitHubToken string

	GitAuthorName  string
	GitAuthorEmail string
}

// Load reads the environment and fills in defaults.
func Load() *Config {
	return &Config{
		Addr:           getEnv("UPSTREAM_ADDR", ":8080"),
		LogLevel:       parseLogLevel(getEnv("LOG_LEVEL", "info")),
		DataDir:        getEnv("UPSTREAM_DATA_DIR", "./data"),
		Repo:           getEnv("UPSTREAM_REPO", ""),
		Branch:         getEnv("UPSTREAM_BRANCH", "main"),
		AppGlob:        getEnv("UPSTREAM_APP_GLOB", "apps/*/*"),
		Schedule:       getEnv("UPSTREAM_SCHEDULE", "0 */6 * * *"),
		GitHubToken:    getEnv("GITHUB_TOKEN", ""),
		GitAuthorName:  getEnv("UPSTREAM_GIT_AUTHOR_NAME", ""),
		GitAuthorEmail: getEnv("UPSTREAM_GIT_AUTHOR_EMAIL", ""),
	}
}

// Problems lists settings that stop a feature from working, without stopping the server.
func (c *Config) Problems() []error {
	var errs []error
	if c.Repo == "" {
		errs = append(errs, errors.New("UPSTREAM_REPO is not set, so there is nothing to scan"))
	}
	if _, err := cron.ParseStandard(c.Schedule); err != nil {
		errs = append(errs, fmt.Errorf("UPSTREAM_SCHEDULE %q isn't a valid cron expression, so scans only run on demand: %w", c.Schedule, err))
	}
	if c.GitHubToken == "" {
		errs = append(errs, errors.New("GITHUB_TOKEN is not set, so release notes and bump PRs are off"))
	}
	if c.GitAuthorName == "" || c.GitAuthorEmail == "" {
		errs = append(errs, errors.New("UPSTREAM_GIT_AUTHOR_NAME and UPSTREAM_GIT_AUTHOR_EMAIL are not both set, so bump PRs are off"))
	}
	return errs
}

func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func parseLogLevel(s string) slog.Level {
	var l slog.Level
	if err := l.UnmarshalText([]byte(s)); err != nil {
		return slog.LevelInfo
	}
	return l
}
