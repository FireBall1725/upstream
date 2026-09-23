// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

// Package repository holds every SQL statement Upstream runs.
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/fireball1725/upstream/internal/models"
)

type Repo struct {
	db *sql.DB
}

func New(db *sql.DB) *Repo { return &Repo{db: db} }

const ts = time.RFC3339Nano

func parseTime(s string) time.Time {
	t, _ := time.Parse(ts, s)
	return t
}

// keepScans is how many scans the history keeps.
const keepScans = 50

// SaveScan records a finished scan. A successful scan's full result replaces the previous one.
func (r *Repo) SaveScan(ctx context.Context, s models.ScanSummary, result []byte) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if result != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE scans SET result = NULL WHERE result IS NOT NULL`); err != nil {
			return err
		}
	}
	var res any
	if result != nil {
		res = string(result)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO scans (started_at, finished_at, state, error, commit_sha, apps, checked, updates, errors, result)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.StartedAt.Format(ts), s.FinishedAt.Format(ts), s.State, s.Error, s.Commit, s.Apps, s.Checked, s.Updates, s.Errors, res); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM scans WHERE id NOT IN (SELECT id FROM scans ORDER BY id DESC LIMIT ?) AND result IS NULL`, keepScans); err != nil {
		return err
	}
	return tx.Commit()
}

// RecentScans lists scans newest first.
func (r *Repo) RecentScans(ctx context.Context, limit int) ([]models.ScanSummary, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT started_at, finished_at, state, error, commit_sha, apps, checked, updates, errors
		FROM scans ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []models.ScanSummary{}
	for rows.Next() {
		var s models.ScanSummary
		var started, finished string
		if err := rows.Scan(&started, &finished, &s.State, &s.Error, &s.Commit, &s.Apps, &s.Checked, &s.Updates, &s.Errors); err != nil {
			return nil, err
		}
		s.StartedAt, s.FinishedAt = parseTime(started), parseTime(finished)
		out = append(out, s)
	}
	return out, rows.Err()
}

// LatestResult returns the JSON of the newest successful scan, or nil if there isn't one.
func (r *Repo) LatestResult(ctx context.Context) ([]byte, error) {
	var res sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT result FROM scans WHERE result IS NOT NULL ORDER BY id DESC LIMIT 1`).Scan(&res)
	if errors.Is(err, sql.ErrNoRows) || !res.Valid {
		return nil, nil
	}
	return []byte(res.String), err
}

func (r *Repo) AddSkip(ctx context.Context, s models.Skip) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO skips (app_dir, field, version, created_at) VALUES (?, ?, ?, ?)
		ON CONFLICT (app_dir, field, version) DO NOTHING`, s.AppDir, s.Field, s.Version, time.Now().Format(ts))
	return err
}

func (r *Repo) DeleteSkip(ctx context.Context, s models.Skip) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM skips WHERE app_dir = ? AND field = ? AND version = ?`, s.AppDir, s.Field, s.Version)
	return err
}

func (r *Repo) ListSkips(ctx context.Context) ([]models.Skip, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT app_dir, field, version, created_at FROM skips ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []models.Skip{}
	for rows.Next() {
		var s models.Skip
		var created string
		if err := rows.Scan(&s.AppDir, &s.Field, &s.Version, &created); err != nil {
			return nil, err
		}
		s.CreatedAt = parseTime(created)
		out = append(out, s)
	}
	return out, rows.Err()
}

// CreatePR stores a new PR and returns its id.
func (r *Repo) CreatePR(ctx context.Context, p models.PullRequest) (int64, error) {
	items, err := json.Marshal(p.Items)
	if err != nil {
		return 0, err
	}
	now := time.Now().Format(ts)
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO pull_requests (created_at, updated_at, title, branch, state, auto_merge, items)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, now, now, p.Title, p.Branch, p.State, p.AutoMerge, string(items))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdatePR saves a PR's progress.
func (r *Repo) UpdatePR(ctx context.Context, p models.PullRequest) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE pull_requests SET updated_at = ?, state = ?, number = ?, url = ?, error = ? WHERE id = ?`,
		time.Now().Format(ts), p.State, p.Number, p.URL, p.Error, p.ID)
	return err
}

// ListPRs lists PRs newest first.
func (r *Repo) ListPRs(ctx context.Context, limit int) ([]models.PullRequest, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, created_at, updated_at, title, branch, state, auto_merge, number, url, error, items
		FROM pull_requests ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []models.PullRequest{}
	for rows.Next() {
		var p models.PullRequest
		var created, updated, items string
		if err := rows.Scan(&p.ID, &created, &updated, &p.Title, &p.Branch, &p.State, &p.AutoMerge, &p.Number, &p.URL, &p.Error, &items); err != nil {
			return nil, err
		}
		p.CreatedAt, p.UpdatedAt = parseTime(created), parseTime(updated)
		if err := json.Unmarshal([]byte(items), &p.Items); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
