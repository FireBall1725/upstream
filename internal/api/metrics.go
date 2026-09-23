// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

package api

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/fireball1725/upstream/internal/models"
)

// metrics writes the Prometheus text format by hand; five gauges don't need a client library.
func (h *handlers) metrics(w http.ResponseWriter, r *http.Request) {
	res := h.scans.Latest()
	pins := map[string]int{}
	findings := map[string]int{}
	for _, a := range res.Apps {
		for _, p := range a.Pins {
			pins[p.Update]++
		}
		for _, f := range a.Findings {
			findings[string(f.Severity)]++
		}
	}
	openPRs := 0
	if prs, err := h.repo.ListPRs(r.Context(), 200); err == nil {
		for _, p := range prs {
			if p.State == models.PROpen {
				openPRs++
			}
		}
	}

	var b strings.Builder
	gauge := func(name, help string) { fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s gauge\n", name, help, name) }
	labelled := func(name string, m map[string]int, label string) {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "%s{%s=%q} %d\n", name, label, k, m[k])
		}
	}

	gauge("upstream_apps", "Apps found by the latest successful scan.")
	fmt.Fprintf(&b, "upstream_apps %d\n", len(res.Apps))
	gauge("upstream_pins", "Pinned versions by update type: major, minor, patch, rebuild, current, unchecked or error.")
	labelled("upstream_pins", pins, "update")
	gauge("upstream_hygiene_findings", "Repo hygiene findings by severity.")
	labelled("upstream_hygiene_findings", findings, "severity")
	gauge("upstream_open_prs", "Bump PRs Upstream opened that are still open.")
	fmt.Fprintf(&b, "upstream_open_prs %d\n", openPRs)
	if len(res.History) > 0 {
		last := res.History[0]
		ok := 0
		if last.State == models.ScanOK {
			ok = 1
		}
		gauge("upstream_last_scan_success", "1 if the most recent scan succeeded.")
		fmt.Fprintf(&b, "upstream_last_scan_success %d\n", ok)
		gauge("upstream_last_scan_timestamp_seconds", "When the most recent scan finished.")
		fmt.Fprintf(&b, "upstream_last_scan_timestamp_seconds %d\n", last.FinishedAt.Unix())
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(b.String()))
}
