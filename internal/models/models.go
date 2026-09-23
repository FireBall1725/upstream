// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

// Package models holds the types shared by the scan, the bump PRs, storage and the API.
package models

import "time"

// ScanState is where a scan got to.
type ScanState string

const (
	ScanNever   ScanState = "never"
	ScanRunning ScanState = "running"
	ScanOK      ScanState = "ok"
	ScanFailed  ScanState = "failed"
)

// ScanSummary is one finished scan, listed on the Scans tab.
type ScanSummary struct {
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
	State      ScanState `json:"state"`
	Error      string    `json:"error,omitempty"`
	Commit     string    `json:"commit,omitempty"`
	Apps       int       `json:"apps"`
	Checked    int       `json:"checked"`
	Updates    int       `json:"updates"`
	Errors     int       `json:"errors"`
}

// Skip hides one version of one pin until something newer comes out.
type Skip struct {
	AppDir    string    `json:"appDir"`
	Field     string    `json:"field"`
	Version   string    `json:"version"`
	CreatedAt time.Time `json:"createdAt"`
}

// PRState is where a bump PR got to.
type PRState string

const (
	PRQueued  PRState = "queued"
	PRRunning PRState = "running"
	PROpen    PRState = "open"
	PRMerged  PRState = "merged"
	PRClosed  PRState = "closed"
	PRFailed  PRState = "failed"
)

// BumpItem is one version change inside a PR.
type BumpItem struct {
	AppDir string `json:"appDir"`
	App    string `json:"app"`
	Field  string `json:"field"`
	Kind   string `json:"kind"`
	Source string `json:"source"`
	From   string `json:"from"`
	To     string `json:"to"`
	Bump   string `json:"bump"`
}

// PullRequest is a bump PR Upstream opened, or tried to.
type PullRequest struct {
	ID        int64      `json:"id"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	Title     string     `json:"title"`
	Branch    string     `json:"branch"`
	State     PRState    `json:"state"`
	AutoMerge bool       `json:"autoMerge"`
	Number    int        `json:"number,omitempty"`
	URL       string     `json:"url,omitempty"`
	Error     string     `json:"error,omitempty"`
	Items     []BumpItem `json:"items"`
}
