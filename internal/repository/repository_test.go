// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/fireball1725/upstream/internal/db"
	"github.com/fireball1725/upstream/internal/models"
)

func open(t *testing.T) *Repo {
	t.Helper()
	conn, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return New(conn)
}

func TestScans(t *testing.T) {
	r, ctx := open(t), context.Background()
	if res, err := r.LatestResult(ctx); err != nil || res != nil {
		t.Fatalf("empty db: %s %v", res, err)
	}
	now := time.Now()
	ok := models.ScanSummary{StartedAt: now, FinishedAt: now.Add(time.Second), State: models.ScanOK, Commit: "abc", Apps: 3, Updates: 1}
	if err := r.SaveScan(ctx, ok, []byte(`{"n":1}`)); err != nil {
		t.Fatal(err)
	}
	failed := models.ScanSummary{StartedAt: now.Add(time.Minute), FinishedAt: now.Add(time.Minute), State: models.ScanFailed, Error: "boom"}
	if err := r.SaveScan(ctx, failed, nil); err != nil {
		t.Fatal(err)
	}
	if err := r.SaveScan(ctx, ok, []byte(`{"n":2}`)); err != nil {
		t.Fatal(err)
	}

	res, err := r.LatestResult(ctx)
	if err != nil || string(res) != `{"n":2}` {
		t.Errorf("latest result %s %v", res, err)
	}
	list, err := r.RecentScans(ctx, 10)
	if err != nil || len(list) != 3 || list[1].Error != "boom" || list[0].Apps != 3 {
		t.Errorf("recent %+v %v", list, err)
	}
	if !list[2].StartedAt.Equal(now) {
		t.Errorf("time round trip %v vs %v", list[2].StartedAt, now)
	}
}

func TestSkips(t *testing.T) {
	r, ctx := open(t), context.Background()
	s := models.Skip{AppDir: "apps/a/b", Field: "image.tag", Version: "2.0.0"}
	for range 2 {
		if err := r.AddSkip(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	list, err := r.ListSkips(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("skips %+v %v", list, err)
	}
	if err := r.DeleteSkip(ctx, s); err != nil {
		t.Fatal(err)
	}
	if list, _ := r.ListSkips(ctx); len(list) != 0 {
		t.Errorf("still skipped %+v", list)
	}
}

func TestPRs(t *testing.T) {
	r, ctx := open(t), context.Background()
	p := models.PullRequest{Title: "spoolman: 0.26.0 -> 0.26.1", Branch: "upstream/x", State: models.PRQueued, AutoMerge: true,
		Items: []models.BumpItem{{AppDir: "apps/app-spoolman/spoolman", Field: "image.tag", From: "0.26.0", To: "0.26.1"}}}
	id, err := r.CreatePR(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	p.ID, p.State, p.Number, p.URL = id, models.PROpen, 210, "https://github.com/o/r/pull/210"
	if err := r.UpdatePR(ctx, p); err != nil {
		t.Fatal(err)
	}
	list, err := r.ListPRs(ctx, 10)
	if err != nil || len(list) != 1 {
		t.Fatalf("prs %+v %v", list, err)
	}
	got := list[0]
	if got.State != models.PROpen || got.Number != 210 || !got.AutoMerge || len(got.Items) != 1 || got.Items[0].To != "0.26.1" {
		t.Errorf("pr %+v", got)
	}
}
