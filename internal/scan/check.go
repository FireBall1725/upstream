// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

package scan

import (
	"context"
	"sync"

	"github.com/fireball1725/upstream/internal/inventory"
	"github.com/fireball1725/upstream/internal/models"
	"github.com/fireball1725/upstream/internal/sources"
	"github.com/fireball1725/upstream/internal/versions"
)

const (
	UpdateCurrent   = "current"
	UpdateUnchecked = "unchecked"
	UpdateError     = "error"
)

// lookups caps concurrent registry and Helm requests; each source is still fetched once.
const lookups = 8

// skipIndex maps "appDir|field" to the versions skipped for that pin.
func skipIndex(skips []models.Skip) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, s := range skips {
		k := s.AppDir + "|" + s.Field
		if out[k] == nil {
			out[k] = map[string]bool{}
		}
		out[k][s.Version] = true
	}
	return out
}

func checkApps(ctx context.Context, look *sources.Lookup, apps []inventory.App, skips map[string]map[string]bool) {
	sem := make(chan struct{}, lookups)
	var wg sync.WaitGroup
	for i := range apps {
		for j := range apps[i].Pins {
			p := &apps[i].Pins[j]
			skipped := skips[apps[i].Dir+"|"+p.Field]
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				checkPin(ctx, look, p, skipped)
			}()
		}
	}
	wg.Wait()
}

func checkPin(ctx context.Context, look *sources.Lookup, p *inventory.Pin, skipped map[string]bool) {
	if p.Kind == inventory.KindImage && (p.Version == "" || inventory.IsFloating(p.Version)) {
		p.Update, p.Note = UpdateUnchecked, "Floating tag, so there's no pinned version to compare."
		return
	}
	cur, ok := versions.Parse(p.Version)
	if !ok {
		p.Update, p.Note = UpdateUnchecked, "This isn't a version number, so there's nothing to compare it with."
		if p.Kind == inventory.KindChart {
			p.Note = "A version range, not a pin. Chart.lock decides what runs."
		}
		return
	}

	var candidates []string
	appVersions := map[string]string{}
	if p.Kind == inventory.KindChart {
		list, err := look.ChartVersions(ctx, p.ChartRepo, p.Name)
		if err != nil {
			p.Update, p.Note = UpdateError, err.Error()
			return
		}
		for _, v := range list {
			candidates = append(candidates, v.Version)
			appVersions[v.Version] = v.AppVersion
		}
	} else {
		tags, err := look.ImageTags(ctx, p.Image)
		if err != nil {
			p.Update, p.Note = UpdateError, err.Error()
			return
		}
		candidates = tags
	}

	if len(skipped) > 0 {
		kept := candidates[:0:0]
		for _, c := range candidates {
			if !skipped[c] {
				kept = append(kept, c)
			}
		}
		candidates = kept
	}
	latest := versions.Latest(cur, candidates)
	p.Latest, p.LatestAppVersion = latest.Raw, appVersions[latest.Raw]
	if b := versions.BumpOf(cur, latest); b != "" {
		p.Update = string(b)
	} else {
		p.Update = UpdateCurrent
	}
}
