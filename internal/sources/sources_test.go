// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

package sources

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// Shaped like a real index: annotations with nested keys and a multi-line string that
// mentions "version:", dependencies with their own versions, and two charts.
const index = `apiVersion: v1
entries:
  traefik:
  - annotations:
      artifacthub.io/changes: "- bump
        version: 99.0.0 in the docs"
    apiVersion: v2
    appVersion: v3.7.13
    dependencies:
    - name: crds
      version: 1.0.0
    name: traefik
    version: 41.6.0
  - apiVersion: v2
    appVersion: "v3.4.0"
    version: "35.4.0"
  hub-manager:
  - version: 1.0.0
    appVersion: v0.45.1
generated: "2026-09-22T00:00:00Z"
`

func TestParseIndex(t *testing.T) {
	got, err := parseIndex(strings.NewReader(index))
	if err != nil {
		t.Fatal(err)
	}
	tr := got["traefik"]
	if len(tr) != 2 || tr[0] != (ChartVersion{"41.6.0", "v3.7.13"}) || tr[1] != (ChartVersion{"35.4.0", "v3.4.0"}) {
		t.Errorf("traefik %+v", tr)
	}
	if hm := got["hub-manager"]; len(hm) != 1 || hm[0].Version != "1.0.0" || hm[0].AppVersion != "v0.45.1" {
		t.Errorf("hub-manager %+v", hm)
	}
	if len(got) != 2 {
		t.Errorf("charts %v", got)
	}
}

// tailscale's index, with 4-space chart names and 8-space list items.
const wideIndex = `apiVersion: v2
entries:
    tailscale-operator:
        - apiVersion: v2
          name: tailscale-operator
          version: 1.102.4
          appVersion: v1.102.4
          urls:
            - tailscale-operator-1.102.4.tgz
        - version: 1.94.2
          appVersion: v1.94.2
generated: 2026-09-17T21:28:57Z
`

func TestParseIndexWideIndent(t *testing.T) {
	got, err := parseIndex(strings.NewReader(wideIndex))
	if err != nil {
		t.Fatal(err)
	}
	ts := got["tailscale-operator"]
	if len(ts) != 2 || ts[0] != (ChartVersion{"1.102.4", "v1.102.4"}) || ts[1] != (ChartVersion{"1.94.2", "v1.94.2"}) {
		t.Errorf("tailscale-operator %+v", ts)
	}
}

func TestChartVersionsFetchesEachIndexOnce(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/charts/index.yaml" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(index))
	}))
	defer srv.Close()

	l := New("upstream-test")
	for _, chart := range []string{"traefik", "hub-manager", "traefik"} {
		if _, err := l.ChartVersions(context.Background(), srv.URL+"/charts/", chart); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := l.ChartVersions(context.Background(), srv.URL+"/charts", "missing"); err == nil {
		t.Error("a chart the index doesn't list should be an error")
	}
	if n := hits.Load(); n != 2 {
		// One fetch per distinct repo URL string: "/charts/" and "/charts".
		t.Errorf("index fetched %d times, want 2", n)
	}
}

func TestRegistryName(t *testing.T) {
	for in, want := range map[string]string{
		"lscr.io/linuxserver/sonarr": "ghcr.io/linuxserver/sonarr",
		"ghcr.io/esphome/esphome":    "ghcr.io/esphome/esphome",
		"nginx":                      "nginx",
	} {
		if got := registryName(in); got != want {
			t.Errorf("registryName(%q) = %q, want %q", in, got, want)
		}
	}
}
